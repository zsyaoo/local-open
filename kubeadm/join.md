kubeadm join 包括master加入到已有集群和worker 加入到已有集群，所以kubeadm join 需要分角色讲解。

当加入master节点 到集群时，使用 kubeadm join –control-plane，–control-plane 表示在此节点上创建一个新的控制平面

### 执行顺序

kubeadm join 的步骤会比init 少很多

- 节点检查，基本和 init 差不多
- 从集群里下载集群相关信息，kubeadm-config.yaml，kubelet-config.yaml，证书，kubeconfig，即kubeadm init 上传的内容
- 检查etcd 是否需要本地部署
- 启动kubelet
- 添加etcd 成员
- 更新集群状态

```go
// NewCmdJoin returns "kubeadm join" command.
// NB. joinOptions is exposed as parameter for allowing unit testing of
//     the newJoinData method, that implements all the command options validation logic
func NewCmdJoin(out io.Writer, joinOptions *joinOptions) *cobra.Command {
	if joinOptions == nil {
		joinOptions = newJoinOptions()
	}
	joinRunner := workflow.NewRunner()

	cmd := &cobra.Command{
		Use:   "join [api-server-endpoint]",
		Short: "Run this on any machine you wish to join an existing cluster",
		Long:  joinLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {

			c, err := joinRunner.InitData(args)
			if err != nil {
				return err
			}

			data := c.(*joinData)

			if err := joinRunner.Run(args); err != nil {
				return err
			}

			// if the node is hosting a new control plane instance
			if data.cfg.ControlPlane != nil {
				// outputs the join control plane done message and exit
				etcdMessage := ""
				if data.initCfg.Etcd.External == nil {
					etcdMessage = "* A new etcd member was added to the local/stacked etcd cluster."
				}

				ctx := map[string]string{
					"KubeConfigPath": kubeadmconstants.GetAdminKubeConfigPath(),
					"etcdMessage":    etcdMessage,
				}
				if err := joinControPlaneDoneTemp.Execute(data.outputWriter, ctx); err != nil {
					return err
				}

			} else {
				// otherwise, if the node joined as a worker node;
				// outputs the join done message and exit
				fmt.Fprint(data.outputWriter, joinWorkerNodeDoneMsg)
			}

			return nil
		},
		// We accept the control-plane location as an optional positional argument
		Args: cobra.MaximumNArgs(1),
	}

	addJoinConfigFlags(cmd.Flags(), joinOptions.externalcfg)
	addJoinOtherFlags(cmd.Flags(), joinOptions)

	joinRunner.AppendPhase(phases.NewPreflightPhase())
	joinRunner.AppendPhase(phases.NewControlPlanePreparePhase())
	joinRunner.AppendPhase(phases.NewCheckEtcdPhase())
	joinRunner.AppendPhase(phases.NewKubeletStartPhase())
	joinRunner.AppendPhase(phases.NewControlPlaneJoinPhase())

	// sets the data builder function, that will be used by the runner
	// both when running the entire workflow or single phases
	joinRunner.SetDataInitializer(func(cmd *cobra.Command, args []string) (workflow.RunData, error) {
		return newJoinData(cmd, args, joinOptions, out)
	})

	// binds the Runner to kubeadm join command by altering
	// command help, adding --skip-phases flag and by adding phases subcommands
	joinRunner.BindToCommand(cmd)

	return cmd
}
```

### Preflight

为部署之前做一些节点检查，如果是node节点只执行RunJoinNodeChecks，如果是控制实例，即master节点，会继续执行checkIfReadyForAdditionalControlPlane，然后执行checkIfReadyForAdditionalControlPlane，RunInitNodeChecks，RunPullImagesCheck

```go
// runPreflight executes preflight checks logic.
func runPreflight(c workflow.RunData) error {
	j, ok := c.(JoinData)
	if !ok {
		return errors.New("preflight phase invoked with an invalid data struct")
	}
	fmt.Println("[preflight] Running pre-flight checks")

	// Start with general checks
	klog.V(1).Infoln("[preflight] Running general checks")
	if err := preflight.RunJoinNodeChecks(utilsexec.New(), j.Cfg(), j.IgnorePreflightErrors()); err != nil {
		return err
	}

	initCfg, err := j.InitCfg()
	if err != nil {
		return err
	}

	// Continue with more specific checks based on the init configuration
	klog.V(1).Infoln("[preflight] Running configuration dependant checks")
	if j.Cfg().ControlPlane != nil {
		// Checks if the cluster configuration supports
		// joining a new control plane instance and if all the necessary certificates are provided
		hasCertificateKey := len(j.CertificateKey()) > 0
		if err := checkIfReadyForAdditionalControlPlane(&initCfg.ClusterConfiguration, hasCertificateKey); err != nil {
			// outputs the not ready for hosting a new control plane instance message
			ctx := map[string]string{
				"Error": err.Error(),
			}

			var msg bytes.Buffer
			notReadyToJoinControlPlaneTemp.Execute(&msg, ctx)
			return errors.New(msg.String())
		}

		// run kubeadm init preflight checks for checking all the prerequisites
		fmt.Println("[preflight] Running pre-flight checks before initializing the new control plane instance")

		if err := preflight.RunInitNodeChecks(utilsexec.New(), initCfg, j.IgnorePreflightErrors(), true, hasCertificateKey); err != nil {
			return err
		}

		fmt.Println("[preflight] Pulling images required for setting up a Kubernetes cluster")
		fmt.Println("[preflight] This might take a minute or two, depending on the speed of your internet connection")
		fmt.Println("[preflight] You can also perform this action in beforehand using 'kubeadm config images pull'")
		if err := preflight.RunPullImagesCheck(utilsexec.New(), initCfg, j.IgnorePreflightErrors()); err != nil {
			return err
		}
	}
	return nil
}
```

RunJoinNodeChecks，进行 node 节点的一些检查

- 检查该节点是否存在 /etc/kubernetes/manifest 目录
- 检查该节点是否存在 /etc/kubernetes/kubelet.conf
- 检查该节点是否存在 /etc/kubernetes/`bootstrap-kubelet.conf`
- 一些通用检查，防火墙，swap是否关闭等
- 检查Bridge-netfilter and IPv6 内核参数是否开启

```go
// RunJoinNodeChecks executes all individual, applicable to node checks.
func RunJoinNodeChecks(execer utilsexec.Interface, cfg *kubeadmapi.JoinConfiguration, ignorePreflightErrors sets.String) error {
	// First, check if we're root separately from the other preflight checks and fail fast
	if err := RunRootCheckOnly(ignorePreflightErrors); err != nil {
		return err
	}

	checks := []Checker{
		DirAvailableCheck{Path: filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.ManifestsSubDirName)},
		FileAvailableCheck{Path: filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.KubeletKubeConfigFileName)},
		FileAvailableCheck{Path: filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.KubeletBootstrapKubeConfigFileName)},
	}
	checks = addCommonChecks(execer, "", &cfg.NodeRegistration, checks)
	if cfg.ControlPlane == nil {
		checks = append(checks, FileAvailableCheck{Path: cfg.CACertPath})
	}

	addIPv6Checks := false
	if cfg.Discovery.BootstrapToken != nil {
		ipstr, _, err := net.SplitHostPort(cfg.Discovery.BootstrapToken.APIServerEndpoint)
		if err == nil {
			checks = append(checks,
				HTTPProxyCheck{Proto: "https", Host: ipstr},
			)
			if ip := net.ParseIP(ipstr); ip != nil {
				if utilsnet.IsIPv6(ip) {
					addIPv6Checks = true
				}
			}
		}
	}
	if addIPv6Checks {
		checks = append(checks,
			FileContentCheck{Path: bridgenf6, Content: []byte{'1'}},
			FileContentCheck{Path: ipv6DefaultForwarding, Content: []byte{'1'}},
		)
	}

	return RunChecks(checks, os.Stderr, ignorePreflightErrors)
}
```

`checkIfReadyForAdditionalControlPlane`， 检查集群是否能够加入其它master，如果没有设置 certificate-key 参数，kubeadm 会认为该节点不会向apiserver 获取证书，即认为该节点已存在证书，则会检查本地是否存在与集群的证书一致。

certificate-key 用来解密证书

```go
// checkIfReadyForAdditionalControlPlane ensures that the cluster is in a state that supports
// joining an additional control plane instance and if the node is ready to preflight
func checkIfReadyForAdditionalControlPlane(initConfiguration *kubeadmapi.ClusterConfiguration, hasCertificateKey bool) error {
	// blocks if the cluster was created without a stable control plane endpoint
	if initConfiguration.ControlPlaneEndpoint == "" {
		return errors.New("unable to add a new control plane instance a cluster that doesn't have a stable controlPlaneEndpoint address")
	}
	// 如果没有设置 certificate-key 参数，kubeadm 会认为该节点不会向apiserver 获取证书，即认为该节点已存在证书，则会检查本地是否存在与集群的证书一致。
	if !hasCertificateKey {
		// checks if the certificates that must be equal across controlplane instances are provided
		if ret, err := certs.SharedCertificateExists(initConfiguration); !ret {
			return err
		}
	}

	return nil
}
```

```
RunInitNodeChecks
```

和init 检查一样

```
RunPullImagesCheck
```

检查节点镜像是否存在，不存在就会拉取。

### ControlPlanePrepare

- 如果是加入控制节点，则加载集群中”kubeadm-certs” secret
- 使用加载的ca证书创建对应所需证书
- 创建kubeconfig
- 创建静态pod manifest

### 1、DownloadCertsSubphase

从集群加载 kubeadm-certs secret

```go
func runControlPlanePrepareDownloadCertsPhaseLocal(c workflow.RunData) error {
	data, ok := c.(JoinData)
	if !ok {
		return errors.New("download-certs phase invoked with an invalid data struct")
	}

  // 如果该节点不是控制节点或者没有传递CertificateKey参数，就不会加载secret
	if data.Cfg().ControlPlane == nil || len(data.CertificateKey()) == 0 {
		klog.V(1).Infoln("[download-certs] Skipping certs download")
		return nil
	}

	cfg, err := data.InitCfg()
	if err != nil {
		return err
	}
  // 获取k8s client，用于加载secret
  // 这个bootstrapClient 可以拿出来看下，是如何通过token获取k8s client
	client, err := bootstrapClient(data)
	if err != nil {
		return err
	}
  // 加载证书到本地磁盘
	if err := copycerts.DownloadCerts(client, cfg, data.CertificateKey()); err != nil {
		return errors.Wrap(err, "error downloading certs")
	}
	return nil
}
```

bootstrapClient(data) 解析

```go
func bootstrapClient(data JoinData) (clientset.Interface, error) {
  // 通过 TLSBootstrapCfg 生成 client
	tlsBootstrapCfg, err := data.TLSBootstrapCfg()
	if err != nil {
		return nil, errors.Wrap(err, "unable to access the cluster")
	}
	client, err := kubeconfigutil.ToClientSet(tlsBootstrapCfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to access the cluster")
	}
	return client, nil
}
// 这个函数会根据kubeadm-join-config 里的Discovery 段判断如何生成client

// DiscoverValidatedKubeConfig returns a validated Config object that specifies where the cluster is and the CA cert to trust
func DiscoverValidatedKubeConfig(cfg *kubeadmapi.JoinConfiguration) (*clientcmdapi.Config, error) {
	switch {
	case cfg.Discovery.File != nil:
		kubeConfigPath := cfg.Discovery.File.KubeConfigPath
		if isHTTPSURL(kubeConfigPath) {
			return https.RetrieveValidatedConfigInfo(kubeConfigPath, kubeadmapiv1beta2.DefaultClusterName, cfg.Discovery.Timeout.Duration)
		}
		return file.RetrieveValidatedConfigInfo(kubeConfigPath, kubeadmapiv1beta2.DefaultClusterName, cfg.Discovery.Timeout.Duration)
  // 因为kubeadm-join-config 里使用token 所以会执行这一步，进而利用discovery.bootstrapToken.token 生成client
	case cfg.Discovery.BootstrapToken != nil:
		return token.RetrieveValidatedConfigInfo(&cfg.Discovery)
	default:
		return nil, errors.New("couldn't find a valid discovery configuration")
	}
}
```

### 2、PrepareCertsPhase

刚刚下载的证书这是集群的ca，这步就是使用这些ca签署对应证书

```go
func runControlPlanePrepareCertsPhaseLocal(c workflow.RunData) error {
	data, ok := c.(JoinData)
	if !ok {
		return errors.New("control-plane-prepare phase invoked with an invalid data struct")
	}

	// 如果不是控制节点即不用创建
	if data.Cfg().ControlPlane == nil {
		return nil
	}

	cfg, err := data.InitCfg()
	if err != nil {
		return err
	}

	fmt.Printf("[certs] Using certificateDir folder %q\n", cfg.CertificatesDir)

	// Generate missing certificates (if any)
	return certsphase.CreatePKIAssets(cfg)
}
```

### 3、PrepareKubeconfigSubphase

创建 kubeconfig

- admin.conf
- kube-controller-manager.conf
- kube-scheduler.conf

```go
func runControlPlanePrepareKubeconfigPhaseLocal(c workflow.RunData) error {
	data, ok := c.(JoinData)
	if !ok {
		return errors.New("control-plane-prepare phase invoked with an invalid data struct")
	}

	// Skip if this is not a control plane
	if data.Cfg().ControlPlane == nil {
		return nil
	}

	cfg, err := data.InitCfg()
	if err != nil {
		return err
	}

	fmt.Println("[kubeconfig] Generating kubeconfig files")
	fmt.Printf("[kubeconfig] Using kubeconfig folder %q\n", kubeadmconstants.KubernetesDir)

	// Generate kubeconfig files for controller manager, scheduler and for the admin/kubeadm itself
	// NB. The kubeconfig file for kubelet will be generated by the TLS bootstrap process in
	// following steps of the join --control-plane workflow
	if err := kubeconfigphase.CreateJoinControlPlaneKubeConfigFiles(kubeadmconstants.KubernetesDir, cfg); err != nil {
		return errors.Wrap(err, "error generating kubeconfig files")
	}

	return nil
}
```

### 4、PrepareControlPlaneSubphase

创建manifest

- kube-apiserver.yaml
- kube-controller-manager.yaml
- kube-scheduler.yaml

```go
func runControlPlanePrepareControlPlaneSubphase(c workflow.RunData) error {
	data, ok := c.(JoinData)
	if !ok {
		return errors.New("control-plane-prepare phase invoked with an invalid data struct")
	}

	// Skip if this is not a control plane
	if data.Cfg().ControlPlane == nil {
		return nil
	}

	cfg, err := data.InitCfg()
	if err != nil {
		return err
	}

	fmt.Printf("[control-plane] Using manifest folder %q\n", kubeadmconstants.GetStaticPodDirectory())
	for _, component := range kubeadmconstants.ControlPlaneComponents {
		fmt.Printf("[control-plane] Creating static Pod manifest for %q\n", component)
		err := controlplane.CreateStaticPodFiles(
			kubeadmconstants.GetStaticPodDirectory(),
			data.KustomizeDir(),
			data.PatchesDir(),
			&cfg.ClusterConfiguration,
			&cfg.LocalAPIEndpoint,
			component,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
```

### CheckEtcd

检查etcd 集群是否健康

```go
func runCheckEtcdPhase(c workflow.RunData) error {
	data, ok := c.(JoinData)
	if !ok {
		return errors.New("check-etcd phase invoked with an invalid data struct")
	}

	// 如果不是控制节点则不检查
	if data.Cfg().ControlPlane == nil {
		return nil
	}

	cfg, err := data.InitCfg()
	if err != nil {
		return err
	}

 // 如果etcd是外部的，即已经有etcd集群了，不检查
	if cfg.Etcd.External != nil {
		fmt.Println("[check-etcd] Skipping etcd check in external mode")
		return nil
	}

	fmt.Println("[check-etcd] Checking that the etcd cluster is healthy")

	// 从该节点的/etc/kubernetes/admin.conf 获取k8s client
	client, err := data.ClientSet()
	if err != nil {
		return err
	}
  // 执行检查动作
	return etcdphase.CheckLocalEtcdClusterStatus(client, &cfg.ClusterConfiguration)
}
```

### KubeletStart

启动 tls bootstrap，启动kubelet

```go
func runKubeletStartJoinPhase(c workflow.RunData) (returnErr error) {
	cfg, initCfg, tlsBootstrapCfg, err := getKubeletStartJoinData(c)
	if err != nil {
		return err
	}
  // /etc/kubernetes/bootstrap-kubelet.conf
	bootstrapKubeConfigFile := kubeadmconstants.GetBootstrapKubeletKubeConfigPath()

	// 函数返回时直接删除该文件，因为这个文件只是临时使用
	defer os.Remove(bootstrapKubeConfigFile)

	// 将 tlsBootstrapCfg 内容写入/etc/kubernetes/bootstrap-kubelet.conf
  // tlsBootstrapCfg 内容在join 命令行解析时已生成，通过discovery.bootStrapToken.token生成，该token和上面加载kubeadm-certs token可以一致。通过 kubeadm token create 即可创建，会根据随机字符串创建token，且该token会设置默认选项
  // kubeadm token create 创建token 发现没有创建对应rbac，这是因为在kubeadm init 的upload cert以及bootstrapToken 两块已经为对应组创建了rbac，所以只要这个token所在组为“system:bootstrappers:kubeadm:default-node-token”即可。
	klog.V(1).Infof("[kubelet-start] writing bootstrap kubelet config file at %s", bootstrapKubeConfigFile)
	if err := kubeconfigutil.WriteToDisk(bootstrapKubeConfigFile, tlsBootstrapCfg); err != nil {
		return errors.Wrap(err, "couldn't save bootstrap-kubelet.conf to disk")
	}

	// 从 tlsBootstrapCfg 加载ca 内容并写入磁盘
	cluster := tlsBootstrapCfg.Contexts[tlsBootstrapCfg.CurrentContext].Cluster
	if _, err := os.Stat(cfg.CACertPath); os.IsNotExist(err) {
		klog.V(1).Infof("[kubelet-start] writing CA certificate at %s", cfg.CACertPath)
		if err := certutil.WriteCert(cfg.CACertPath, tlsBootstrapCfg.Clusters[cluster].CertificateAuthorityData); err != nil {
			return errors.Wrap(err, "couldn't save the CA certificate to disk")
		}
	}
  // 通过 /etc/kubernetes/bootstrap-kubelet.conf 获取k8s client
	bootstrapClient, err := kubeconfigutil.ClientSetFromFile(bootstrapKubeConfigFile)
	if err != nil {
		return errors.Errorf("couldn't create client from kubeconfig file %q", bootstrapKubeConfigFile)
	}

	// 获取节点hostname
	nodeName, _, err := kubeletphase.GetNodeNameAndHostname(&cfg.NodeRegistration)
	if err != nil {
		klog.Warning(err)
	}

	// 如果集群已存在该节点，则退出
	klog.V(1).Infof("[kubelet-start] Checking for an existing Node in the cluster with name %q and status %q", nodeName, v1.NodeReady)
	node, err := bootstrapClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "cannot get Node %q", nodeName)
	}
	for _, cond := range node.Status.Conditions {
		if cond.Type == v1.NodeReady && cond.Status == v1.ConditionTrue {
			return errors.Errorf("a Node with name %q and status %q already exists in the cluster. "+
				"You must delete the existing Node or change the name of this new joining Node", nodeName, v1.NodeReady)
		}
	}

	// Configure the kubelet. In this short timeframe, kubeadm is trying to stop/restart the kubelet
	// Try to stop the kubelet service so no race conditions occur when configuring it
	klog.V(1).Infoln("[kubelet-start] Stopping the kubelet")
	kubeletphase.TryStopKubelet()

	// 将kubeadm-join-config.yaml里的kubelet部分写入/var/lib/kubelet/config.yaml
	if err := kubeletphase.WriteConfigToDisk(&initCfg.ClusterConfiguration, kubeadmconstants.KubeletRunDirectory); err != nil {
		return err
	}

	// 写入/var/lib/kubelet/kubeadm-flags.env
	registerTaintsUsingFlags := cfg.ControlPlane == nil
	if err := kubeletphase.WriteKubeletDynamicEnvFile(&initCfg.ClusterConfiguration, &initCfg.NodeRegistration, registerTaintsUsingFlags, kubeadmconstants.KubeletRunDirectory); err != nil {
		return err
	}

	// 启动kubelet
	fmt.Println("[kubelet-start] Starting the kubelet")
	kubeletphase.TryStartKubelet()

	// 等待kubelet.conf 授权成功
	waiter := apiclient.NewKubeWaiter(nil, kubeadmconstants.TLSBootstrapTimeout, os.Stdout)
	if err := waiter.WaitForKubeletAndFunc(waitForTLSBootstrappedClient); err != nil {
		fmt.Printf(kubeadmJoinFailMsg, err)
		return err
	}

	// When we know the /etc/kubernetes/kubelet.conf file is available, get the client
	client, err := kubeconfigutil.ClientSetFromFile(kubeadmconstants.GetKubeletKubeConfigPath())
	if err != nil {
		return err
	}

	klog.V(1).Infoln("[kubelet-start] preserving the crisocket information for the node")
	if err := patchnodephase.AnnotateCRISocket(client, cfg.NodeRegistration.Name, cfg.NodeRegistration.CRISocket); err != nil {
		return errors.Wrap(err, "error uploading crisocket")
	}

	return nil
}
```

```go
// 设置token 默认
func SetDefaults_BootstrapToken(bt *bootstraptokenv1.BootstrapToken) {
   if bt.TTL == nil {
      bt.TTL = &metav1.Duration{
         Duration: constants.DefaultTokenDuration,
      }
   }
   if len(bt.Usages) == 0 {
      bt.Usages = constants.DefaultTokenUsages
   }

   if len(bt.Groups) == 0 {
      bt.Groups = constants.DefaultTokenGroups
   }
}
```

### ControlPlaneJoin

包括 生成 etcd 静态pod文件，更新集群状态，如果该节点是控制实例，给该节点打上污点

```go
func NewControlPlaneJoinPhase() workflow.Phase {
	return workflow.Phase{
		Name:    "control-plane-join",
		Short:   "Join a machine as a control plane instance",
		Example: controlPlaneJoinExample,
		Phases: []workflow.Phase{
			{
				Name:           "all",
				Short:          "Join a machine as a control plane instance",
				InheritFlags:   getControlPlaneJoinPhaseFlags("all"),
				RunAllSiblings: true,
				ArgsValidator:  cobra.NoArgs,
			},
			newEtcdLocalSubphase(),
			newUpdateStatusSubphase(),
			newMarkControlPlaneSubphase(),
		},
	}
}
```

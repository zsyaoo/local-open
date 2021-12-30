### 一、流程介绍

![image](https://user-images.githubusercontent.com/23715258/147734002-3b498368-efe7-4a9a-81da-4631937d05b6.png)
 kubeadm 在执行 init 的过程中，主要包含：配置加载、环境检测、集群初始化、安装后配置等步骤。

       配置初始化指的是加载系统默认参数、解析和应用用户指定的配置、设置动态配置项（如节点名称、节点 IP 等）以及验证配置有效性等过程。配置的初始化过程很大程度上依赖 cobra CLI 解析工具实现，关于 cobra 的使用请参考博文K8S 源码探秘 之 命令行解析工具 cobra；默认配置的加载过程介绍请参考博文K8S 源码探秘 之 默认参数的加载过程（Scheme 初了解）。
    
       环境检测涉及的东西较多，我将其独立成一个章节进行介绍，参见 3.1 节。
    
       镜像检测和拉取指的是 kubeadm 在执行集群初始化前，会首先检查本地是否包含系统运行所需的基础镜像，如果没有则通过网络从公有仓库拉取，这些镜像必须的包括：kube-apiserver、kube-controller-manager、kube-scheduler、kube-proxy以及pause。此外，当使用 Local etcd 时，kubeadm 会额外拉取 etcd 镜像。至于 DNS，CoreDNS 和 KubeDNS 是二选一的，根据配置进行相应镜像的拉取。使用 CoreDNS 只需要拉取一个镜像，即 coredns；而使用 KubeDNS 时则需要拉取三个镜像，分别是：k8s-dns-kube-dns、k8s-dns-sidecard、k8s-dns-dnsmasq-nanny。
       
        配置本地 kubelet 服务指的是根据集群初始化配置对本地的 kubelet 服务重新进行配置的过程。首先，kubeadm 会停止本地的 kubelet 服务；而后配置两个文件，即 /var/lib/kubelet/kubeadm-flags.env 和 /var/lib/kubelet/config.yaml，这两个文件声明了本地 kubelet 启动需要应用的配置参数；最后重启 kubelet 服务。
    
      检测和生成相关证书指的是使用用户指定的 CA 或者采用自签名的方式生成 k8s 各组件的证书文件，以便各组件通信时使用安全的连接。生成的证书文件存放在 /etc/kubernetes/pki 目录下，各服务组件对应的配置则存放在 /etc/kubernetes 目录下，分别为：admin.conf、kubelet.conf、controller-manager.conf、scheduler.conf。
      
       检测和生成 audit 策略文件，该功能需要在配置参数里启用 Auditing 特性才会执行。关于 Auditing 的相关说明，请参照官网：https://kubernetes.io/docs/tasks/debug-application-cluster/audit/。简单来说，该步骤就是检查一下用户是否提供了 audit 策略文件，如果没有指定则自动创建一个，路径 /etc/kubernetes/audit/audit.yaml。该文件指明审计功能应该记录哪些内容。
       
        生成 manifest 文件指的是生成 Control Plane 各服务组件的运行说明文件，包括 kube-apiserver.yaml、kube-controller-manager.yaml、kube-scheduler.yaml，如果需要运行 Local etcd，则同时生成 etcd.yaml，这些文件都位于 kubernetes 配置目录 /etc/kubernetes/manifests/ 下。当这些文件生成后，kubelet 会自动检测到，从而以容器形式启动相应的服务。
        
        当服务开始运行后，kubeadm 会创建一个 kube-apiserver 的 client，以一定间隔时间持续尝试连接 apiserver，直到探测到 apiserver 是正常运行的，或者重试超时宣告初始化失败。与此同时，kubeadm 也会持续检查 kubelet 的运行状态是否正常，直到探测成功，或者重试超过指定次数宣告初始化失败。这也就是等待 Control Plane 启动完成的过程了。
    
          待主体服务运行起来后，kubeadm 会执行一系列的后续操作，该部分独立成一个章节进行介绍，参见 3.2 节。

### 二、初始化过程

#### 2.1  安装预检

       kubeadm 在执行安装之前进行了相当细致的环境检测，下面就来扒一朳：
    
       1) 检查执行 init 命令的用户是否为 root，如果不是 root，直接快速失败（fail fast）；
    
       2) 检查待安装的 k8s 版本是否被当前版本的 kubeadm 支持（kubeadm 版本 >= 待安装 k8s 版本）；
    
       3) 检查防火墙，如果防火墙未关闭，提示开放端口 10250；
    
       4) 检查端口是否已被占用，6443（或你指定的监听端口）、10251、10252；
    
       5) 检查文件是否已经存在，/etc/kubernetes/manifests/*.yaml；
    
       6) 检查是否存在代理，连接本机网络、服务网络、Pod网络，都会检查，目前不允许代理；
    
       7) 检查容器运行时，使用 CRI 还是 Docker，如果是 Docker，进一步检查 Docker 服务是否已启动，是否设置了开机自启动；
    
       8) 对于 Linux 系统，会额外检查以下内容：
    
           8.1) 检查以下命令是否存在：crictl、ip、iptables、mount、nsenter、ebtables、ethtool、socat、tc、touch；
    
           8.2) 检查 /proc/sys/net/bridge/bridge-nf-call-iptables、/proc/sys/net/ipv4/ip-forward 内容是否为 1；
    
           8.3) 检查 swap 是否是关闭状态；
    
        9) 检查内核是否被支持，Docker 版本及后端存储 GraphDriver 是否被支持；
    
             对于 Linux 系统，还需检查 OS 版本和 cgroup 支持程度（支持哪些资源的隔离）；
    
        10) 检查主机名访问可达性；
    
        11) 检查 kubelet 版本，要高于 kubeadm 需要的最低版本，同时不高于待安装的 k8s 版本；
    
        12) 检查 kubelet 服务是否开机自启动；
    
        13) 检查 10250 端口是否被占用；
    
        14) 如果开启 IPVS 功能，检查系统内核是否加载了 ipvs 模块；
    
        15) 对于 etcd，如果使用 Local etcd，则检查 2379 端口是否被占用， /var/lib/etcd/ 是否为空目录；
    
              如果使用 External etcd，则检查证书文件是否存在（CA、key、cert），验证 etcd 服务版本是否符合要求；
    
        16) 如果使用 IPv6，
    
               检查 /proc/sys/net/bridge/bridge-nf-call-iptables、/proc/sys/net/ipv6/conf/default/forwarding 内容是否为 1；
    
          以上就是 kubeadm init 需要检查的所有项目了！

#### 2.2  完成安装前的配置

       1) 在 kube-system 命名空间创建 ConfigMap kubeadm-config，同时对其配置 RBAC 权限；
    
       2) 在 kube-system 命名空间创建 ConfigMap kubelet-config-<version>，同时对其配置 RBAC 权限；
       
       3) 为当前节点（Master）打标记：node-role.kubernetes.io/master=；
    
       4) 为当前节点（Master）补充 Annotation；
    
       5) 如果启用了 DynamicKubeletConfig 特性，设置本节点 kubelet 的配置数据源为 ConfigMap 形式；
    
       6) 创建 BootStrap token Secret，并对其配置 RBAC 权限；
    
       7) 在 kube-public 命名空间创建 ConfigMap cluster-info，同时对其配置 RBAC 权限；
    
       8) 与 apiserver 通信，部署 DNS 服务；
    
       9) 与 apiserver 通信，部署 kube-proxy 服务；
    
       10) 如果启用了 self-hosted 特性，将 Control Plane 转为 DaemonSet 形式运行；
    
       11) 打印 join 语句；
    
        以上就是 kubeadm 最后所做的操作了，为执行 kubeadm join 做好了铺垫。

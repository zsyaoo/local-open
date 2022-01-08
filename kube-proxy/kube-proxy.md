# kube-proxy

## 介绍

kube-proxy是kubernetes中的重要组件，每个节点上都有一个该组件，以daemonset部署。提供service到pod之间的网络转发，具有负载均衡能力。

kube-proxy有三种工作模式：iptables kube-proxy userspace。

- iptables模式：通过配置iptables规则，实现流量的转发。时间复杂度O(n)。在转发规则多时，性能不高。
- ipvs模式：在规则查找时，通过hash算法，时间复杂度 O(1)，解决了大量规则查找时性能问题。但是有自己本身天然的问题：**conn_reuse_mode的参数为0导致的滚动发布时服务访问失败的问题至今（2021年4月）也解决的不太干净**

```shell
# 1，延迟1s
# 0, 不延迟，如果请求 cluster IP 的源端口被重用了，也就是在 conntrack table 里已经有了 <src_ip, src_port, cluster_ip, cluster_port> 条目了，那么 IPVS 会直接选择之前服务这个 <src_ip, src_port> 的 pod，并不会真的按权重走负载均衡逻辑，导致新的连接去了那个要被删除的 pod，当那个 pod 30s 后被删除后，这些重用了源端口的流量就会连接失败了；现象：部分请求会报告连接失败，no route to host。
[root@ssa2 vs]# cat /proc/sys/net/ipv4/vs/conn_reuse_mode
0
```

**思考：Pod有30s后的删除时间，那conntrack条目如果在这个时间内删除，这时在conntrack表中没有这个四元组，请求不就不会过来了吗？**

答：默认是tcp关闭连接10s后，conntrack条目会删除

```
[root@ssa2 netfilter]# cat nf_conntrack_tcp_timeout_close 10
```

但是由于pod在准备删除30s时间内，是有可能有请求进来的，只要有一个请求进来，这个时间就会被拉长10s，直接pod删除后，这个记录最长还可能存在10s。这个时间内就会发生请求来了，pod不存在场景。直到此时，才会彻底不会有这个conntrack条目。

- userspace模式：这个是早期的模式，工作在用户空间，现在基本已经不用。原理是类似一个普通的负载均衡器，每个service连接到后端pod，都需要经过它来转发。在主机上会找开一个新的连接端口，用于和后端通信。**注入sidecar后pods，在通信的时候，跟这种方式很类似。一旦注入sidecar，这个pods的通信就不会走kube-proxy，而是通过envoy直接转发到下一个service ip。service ip通过selector中的label来匹配后端pods，根据pods label匹配不同版本，实现流量的灰度发布**

**问题，能否以static pod方式来部署该组件？**

答案是不行，static pod只在master节点上部署。而该组件要求在所有的节点部署。

## 关于内核参数的优化

在代码中，在真正server run起来之前，需要优化内核参数。

ipvs对内核的操作相关项。

```go
// In IPVS proxy mode, the following flags need to be set
const (
sysctlBridgeCallIPTables= "net/bridge/bridge-nf-call-iptables"
sysctlVSConnTrack= "net/ipv4/vs/conntrack"
sysctlConnReuse= "net/ipv4/vs/conn_reuse_mode"
sysctlExpireNoDestConn= "net/ipv4/vs/expire_nodest_conn"
sysctlExpireQuiescentTemplate= "net/ipv4/vs/expire_quiescent_template"
sysctlForward= "net/ipv4/ip_forward"
sysctlArpIgnore= "net/ipv4/conf/all/arp_ignore"
sysctlArpAnnounce= "net/ipv4/conf/all/arp_announce"
)
```

对于kube-proxy来说，使用的是LVS的NAT模式。

service ip 及 vip，pods是后端，充当RealServer。

需要设置arp ignore 及 arp announce，代码中也是这样来设置的。

```shell
echo 1 > /proc/sys/net/ipv4/conf/all/arp_ignore
echo 2 > /proc/sys/net/ipv4/conf/all/arp_announce
```

关于这两个参数的介绍：

- arp_ignore

arp_ignore参数的作用是控制系统在收到外部的arp请求时，是否要返回arp响应。

arp_ignore参数常用的取值主要有0，1，2，3~8较少用到：

```go
0：响应任意网卡上接收到的对本机IP地址的arp请求（包括环回网卡上的地址），而不管该目的IP是否在接收网卡上。

1：只响应目的IP地址为接收网卡上的本地地址的arp请求。

2：只响应目的IP地址为接收网卡上的本地地址的arp请求，并且arp请求的源IP必须和接收网卡同网段。
```

- arp_announce

arp_announce的作用是控制系统在对外发送arp请求时，如何选择arp请求数据包的源IP地址。（比如系统准备通过网卡发送一个数据包a，这时数据包a的源IP和目的IP一般都是知道的，而根据目的IP查询路由表，发送网卡也是确定的，故源MAC地址也是知道的，这时就差确定目的MAC地址了。而想要获取目的IP对应的目的MAC地址，就需要发送arp请求。arp请求的目的IP自然就是想要获取其MAC地址的IP，而arp请求的源IP是什么呢？ 可能第一反应会以为肯定是数据包a的源IP地址，但是这个也不是一定的，arp请求的源IP是可以选择的，控制这个地址如何选择就是arp_announce的作用）

arp_announce参数常用的取值有0，1，2。

```go
0：允许使用任意网卡上的IP地址作为arp请求的源IP，通常就是使用数据包a的源IP。

1：尽量避免使用不属于该发送网卡子网的本地地址作为发送arp请求的源IP地址。

2：忽略IP数据包的源IP地址，选择该发送网卡上最合适的本地地址作为arp请求的源IP地址。

# sysctl.conf中包含all和eth/lo（具体网卡）的arp_ignore参数，取其中较大的值生效。
```

## 源码阅读

cobra是一个命令行工具，可以使用命令快速创建脚手架，帮助完成命令行执行时参数传入、配置文件读取的一系列操作。

cobra主要是围绕cobra.Command struct展开。

整体实现机制就是： New 出一个实例 ——> 初始化赋值 ——> Run。

其实代码本质都是类似的一个逻辑。

系统初始化后，通过读入命令行参数或是配置文件，把该struct中的各个变量赋值。

最后Run时，调用Command的Run中的函数，该函数主要是真正业务开始的地方。

```go
// NewProxyCommand creates a *cobra.Command object with default parameters
func NewProxyCommand() *cobra.Command {
	// 初始化option变量
	opts := NewOptions()

	cmd := &cobra.Command{
		Use: "kube-proxy",
		Long: `The Kubernetes network proxy runs on each node. This
reflects services as defined in the Kubernetes API on each node and can do simple
TCP, UDP, and SCTP stream forwarding or round robin TCP, UDP, and SCTP forwarding across a set of backends.
Service cluster IPs and ports are currently found through Docker-links-compatible
environment variables specifying ports opened by the service proxy. There is an optional
addon that provides cluster DNS for these cluster IPs. The user must create a service
with the apiserver API to configure the proxy.`,
		// 真正的执行入口是这里
    // cobra Command有个Command struct，把相关的变量初始化
		// Run是该struct的一个成员变量，这里只是传入一个函数，在运行command.run时，才会使用这个函数，才会真正运行起来
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			cliflag.PrintFlags(cmd.Flags())

			// 对系统初始化，会设置文件最大打开数、conntrack相关参数等
			if err := initForOS(opts.WindowsService); err != nil {
				klog.Fatalf("failed OS init: %v", err)
			}

			if err := opts.Complete(); err != nil {
				klog.Fatalf("failed complete: %v", err)
			}
			if err := opts.Validate(); err != nil {
				klog.Fatalf("failed validate: %v", err)
			}

			if err := opts.Run(); err != nil {
				klog.Exit(err)
			}
		},
		...
	}
```

代码一开始，就初始化了cobra.Command，定义了Run函数，但是并没有执行，因此像Flag变量还是默认值。

下一步就是读入Flag相关变量。

flag的传入有两种方式：goflag是官方提供的。另一种方式是pflag，pflag跟cobra一样是spf13提供的；可以直接把goflag接收的参数转换成pflag方式。

pflag实现的原理也很简单：先New一个实例，然后调取os.Args读取配置，把这个实例初始化。

```go
// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
// normalize func and add the go flag set by hand.
pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
```

先是new一个对象出来，options存放了程序运行所有相关的参数。

```
opts := NewOptions()

// 如注释所说，Options包含了运行时所有的相关信息
// Options contains everything necessary to create and run a proxy server.
type Options struct {
	// ConfigFile is the location of the proxy server's configuration file.
	ConfigFile string
	// WriteConfigTo is the path where the default configuration will be written.
	WriteConfigTo string
	// CleanupAndExit, when true, makes the proxy server clean up iptables and ipvs rules, then exit.
	CleanupAndExit bool
	// 如果是windowsService，就不用设置conntrack等内核参数
	// WindowsService should be set to true if kube-proxy is running as a service on Windows.
	// Its corresponding flag only gets registered in Windows builds
	WindowsService bool
	// config is the proxy server's configuration object.
	config *kubeproxyconfig.KubeProxyConfiguration
	// 监测文件，自动重启服务
	// watcher is used to watch on the update change of ConfigFile
	watcher filesystem.FSWatcher
	// 真正干活的是这这个，后面会先new一个对象，然后run
	// proxyServer is the interface to run the proxy server
	proxyServer proxyRun
	// errCh is the channel that errors will be sent
	errCh chan error

	// The fields below here are placeholders for flags that can't be directly mapped into
	// config.KubeProxyConfiguration.
	//
	// TODO remove these fields once the deprecated flags are removed.

	// master is used to override the kubeconfig's URL to the apiserver.
	master string
	// healthzPort is the port to be used by the healthz server.
	healthzPort int32
	// metricsPort is the port to be used by the metrics server.
	metricsPort int32

	// hostnameOverride, if set from the command line flag, takes precedence over the `HostnameOverride` value from the config file
	hostnameOverride string
}
```

Proxy Server会起一个http Server。

```
// ProxyServer represents all the parameters required to start the Kubernetes proxy server. All
// fields are required.
type ProxyServer struct {
	Client                 clientset.Interface
	// EventClient是clientset的一种
	EventClient            v1core.EventsGetter
	IptInterface           utiliptables.Interface
	IpvsInterface          utilipvs.Interface
	IpsetInterface         utilipset.Interface
	execer                 exec.Interface
	// Porxier，ipvs & iptables都会去实例化这个，iptables、main表/local表中的相关路由、ipset等，在这个里面执行
	Proxier                proxy.Provider
	Broadcaster            record.EventBroadcaster
	Recorder               record.EventRecorder
	ConntrackConfiguration kubeproxyconfig.KubeProxyConntrackConfiguration
	Conntracker            Conntracker // if nil, ignored
	// ipvs?iptables?userspace?
	ProxyMode              string
	NodeRef                *v1.ObjectReference
	MetricsBindAddress     string
	BindAddressHardFail    bool
	EnableProfiling        bool
	UseEndpointSlices      bool
	OOMScoreAdj            *int32
	ConfigSyncPeriod       time.Duration
	HealthzServer          healthcheck.ProxierHealthUpdater
}
```

kube-proxy有三种模式，每个模式，都模式都必须有个真正干活的，实现统一的接口。

```
// Proxier is an iptables based proxy for connections between a localhost:lport
// and services that provide the actual backends.
type Proxier struct {
	// endpointsChanges and serviceChanges contains all changes to endpoints and
	// services that happened since iptables was synced. For a single object,
	// changes are accumulated, i.e. previous is state from before all of them,
	// current is state after applying all of those.
	endpointsChanges *proxy.EndpointChangeTracker
	serviceChanges   *proxy.ServiceChangeTracker

	mu           sync.Mutex // protects the following fields
	serviceMap   proxy.ServiceMap
	endpointsMap proxy.EndpointsMap
	portsMap     map[utilnet.LocalPort]utilnet.Closeable
	nodeLabels   map[string]string
	// endpointsSynced, endpointSlicesSynced, and servicesSynced are set to true
	// when corresponding objects are synced after startup. This is used to avoid
	// updating iptables with some partial data after kube-proxy restart.
	endpointsSynced      bool
	endpointSlicesSynced bool
	servicesSynced       bool
	initialized          int32
	syncRunner           *async.BoundedFrequencyRunner // governs calls to syncProxyRules
	syncPeriod           time.Duration

	// These are effectively const and do not need the mutex to be held.
	iptables       utiliptables.Interface
	// 是否把所有的请示snat
	masqueradeAll  bool
	masqueradeMark string
	exec           utilexec.Interface
	localDetector  proxyutiliptables.LocalTrafficDetector
	hostname       string
	nodeIP         net.IP
	portMapper     utilnet.PortOpener
	recorder       record.EventRecorder

	serviceHealthServer healthcheck.ServiceHealthServer
	healthzServer       healthcheck.ProxierHealthUpdater

	// Since converting probabilities (floats) to strings is expensive
	// and we are using only probabilities in the format of 1/n, we are
	// precomputing some number of those and cache for future reuse.
	precomputedProbabilities []string

	// The following buffers are used to reuse memory and avoid allocations
	// that are significantly impacting performance.
	iptablesData             *bytes.Buffer
	existingFilterChainsData *bytes.Buffer
	filterChains             *bytes.Buffer
	filterRules              *bytes.Buffer
	natChains                *bytes.Buffer
	natRules                 *bytes.Buffer

	// endpointChainsNumber is the total amount of endpointChains across all
	// services that we will generate (it is computed at the beginning of
	// syncProxyRules method). If that is large enough, comments in some
	// iptable rules are dropped to improve performance.
	endpointChainsNumber int

	// Values are as a parameter to select the interfaces where nodeport works.
	nodePortAddresses []string
	// networkInterfacer defines an interface for several net library functions.
	// Inject for test purpose.
	networkInterfacer utilproxy.NetworkInterfacer
}
```

同样的套路，new一个实例，然后start起来。

```
func (s *ProxyServer) Run() error {
...
// 创建informer, informer会维护一个本地的资源缓存，get list时，从这里面取，减轻了访问apiserver的压力
// Make informers that filter out objects that want a non-default service proxy.
	informerFactory := informers.NewSharedInformerFactoryWithOptions(s.Client, s.ConfigSyncPeriod,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = labelSelector.String()
		}))

	// 维护service endpoint的一个对应关系，service config controller
	// Create configs (i.e. Watches for Services and Endpoints or EndpointSlices)
	// Note: RegisterHandler() calls need to happen before creation of Sources because sources
	// only notify on changes, and the initial update (on process start) may be lost if no handlers
	// are registered yet.
	serviceConfig := config.NewServiceConfig(informerFactory.Core().V1().Services(), s.ConfigSyncPeriod)
	serviceConfig.RegisterEventHandler(s.Proxier)
	go serviceConfig.Run(wait.NeverStop)

// 创建两个config，serviceConfig endpointConfig，new出来两个实例后，分别run起来
// Create configs (i.e. Watches for Services and Endpoints or EndpointSlices)
	// Note: RegisterHandler() calls need to happen before creation of Sources because sources
	// only notify on changes, and the initial update (on process start) may be lost if no handlers
	// are registered yet.
	serviceConfig := config.NewServiceConfig(informerFactory.Core().V1().Services(), s.ConfigSyncPeriod)
	serviceConfig.RegisterEventHandler(s.Proxier)
	// 这里有个疑问，这个run，为什么可以一直循环处理，没有看明白
	// 这里只是起两个协程来完成下初始化，主要目的是把proxyer注册到handler
	go serviceConfig.Run(wait.NeverStop)

	if s.UseEndpointSlices {
		endpointSliceConfig := config.NewEndpointSliceConfig(informerFactory.Discovery().V1beta1().EndpointSlices(), s.ConfigSyncPeriod)
		endpointSliceConfig.RegisterEventHandler(s.Proxier)
		go endpointSliceConfig.Run(wait.NeverStop)
	} else {
		endpointsConfig := config.NewEndpointsConfig(informerFactory.Core().V1().Endpoints(), s.ConfigSyncPeriod)
		endpointsConfig.RegisterEventHandler(s.Proxier)
		go endpointsConfig.Run(wait.NeverStop)
	}

// 前面全都是初始化各种实例，真正的循环处理在这里
// 该函数有限流的作用，使用的是令牌桶限流
// 会一直循环处理fn函数，即proxier.syncProxyRules，该函数会获取当前所有的ipset iptables信息
// ，并与apiserver的值比较，差异的 
proxier.syncRunner = async.NewBoundedFrequencyRunner("sync-runner", proxier.syncProxyRules, minSyncPeriod, syncPeriod, burstSyncs)
```

上面可以看作是消费者，那生产者是谁呢？

通过k8s的event

```go
// NewServiceConfig creates a new ServiceConfig.
func NewServiceConfig(serviceInformer coreinformers.ServiceInformer, resyncPeriod time.Duration) *ServiceConfig {
	result := &ServiceConfig{
		listerSynced: serviceInformer.Informer().HasSynced,
	}

	serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    result.handleAddService,
			UpdateFunc: result.handleUpdateService,
			DeleteFunc: result.handleDeleteService,
		},
		resyncPeriod,
	)

	return result
}
```


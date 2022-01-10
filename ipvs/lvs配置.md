## 一，LVS (linux virtual server) linux 虚拟服务器的体系架构

### 常见的负载均衡器

根据工作在的协议层划分可划分为：

- 四层负载均衡（位于内核层）：根据请求报文中的目标地址和端口进行调度
- 七层负载均衡（位于应用层）：根据请求报文的内容进行调度，这种调度属于「代理」的方式

根据软硬件划分：

- 硬件负载均衡：
  - F5 的 BIG-IP
  - Citrix 的 NetScaler
  - 这类硬件负载均衡器通常能同时提供四层和七层负载均衡，但同时也价格不菲
- 软件负载均衡：
  - TCP 层：LVS，HaProxy，Nginx
  - 基于 HTTP 协议：Haproxy，Nginx，ATS（Apache Traffic Server），squid，varnish
  - 基于 MySQL 协议：mysql-proxy

lvs是由国人章文嵩开发的一款自由软件,使用 LVS 架设的服务器集群系统有三个部分组成：最前端的负载均衡层（Loader Balancer），中间的服务器群组层，用Server Array表示，最底层的数据共享存储层，用Shared Storage表示。在用户看来所有的应用都是透明的，用户只是在使用一个虚拟服务器提供的高性能服务。见下图：

![2339080](http://cdn.tianfeiyu.com/wp-content/uploads/2016/03/2339080.png)

LVS 是一个工作在四层的负载均衡器，它的实现和 iptables/netfilter 类似，工作在内核空间的 TCP/IP 协议栈上，LVS 工作在 INPUT Hook Funtion 上，并在 INPUT 设置附加规则，一旦客户端请求的是集群服务，LVS 会强行修改请求报文，将报文发往 POSTROUTING，转发至后端的主机。简单来说就是把ip加端口定义为ipvs集群服务，ipvs会为此请求定义一个或多个后端服务，目标地址未必会改，但是报文会被强行转发给后端的服务器。

LVS 的IP负载均衡技术是通过IPVS模块来实现的，即由 ipvsadm（用户空间，用来编写规则） + ipvs（内核空间）两个组件构成的，IPVS是LVS集群系统的核心软件，它的主要作用是：安装在Director Server上，同时在Director Server上虚拟出一个IP地址，用户必须通过这个虚拟的IP地址访问服务。这个虚拟IP一般称为LVS的VIP，即Virtual IP。访问的请求首先经过VIP到达负载调度器，然后由负载调度器从Real Server列表中选取一个服务节点响应用户的请求。LVS 中报文的信息流为： CIP<–>VIP–DIP<–>RIP

```go
1) CIP: Client ip

2) VIP: virtual ip

3) DIP: Director IP

4) RIP: Real server Ip

客户端请求，被VIP端口接收后，从DIP接口被转发出去，并转发至RIP。
```

由于LVS是在内核层做的转发，因此其负载均衡效果非常好，据说其最大能抗住300~400万左右的并发，像 F5 的 BIG-IP 最多也就600 ~700 万的并发量，所以许多大公司都会采用LVS。下图为淘宝网使用LVS的CDN架构图：

![淘宝CND](http://cdn.tianfeiyu.com/wp-content/uploads/2016/03/%E6%B7%98%E5%AE%9DCND.png)

## 二、LVS 的工作模式

### 1，NAT模式

NAT模型,是通过网络地址转换来实现的,他的工作方式是,首先用户请求到达前端的负载均衡器(即Director Server),然后负载均衡器根据事先定义好的调度算法将用户请求的目标地址修改为后端的应用服务器(即Real Server) , 应用程序服务器处理好请求之后将结果返回给用户,期间必须要经过负载均衡器,负载均衡器将报文的源地址 改为用户请求的目标地址,再转发给用户,从而完成整个负载均衡的过程,

![lvs-nat](http://cdn.tianfeiyu.com/wp-content/uploads/2016/03/lvs-nat.png)

```go
NAT(dNAT)访问过程如下:

1，CIP请求VIP，DIP根据调度到的RS修改数据包VIP为RIP并转发给RS

2，RIP响应请求，将数据包源地址改为自己RIP，目标地址为VIP发送给DIP

3，DIP收到后，将源地址改为VIP发送给CIP
```

```go
NAT特性:
1） RS应该使用私有地址

2） RS的网关必须指向DIP

3） RIP 和 DIP 必须在一同意网段内

4） 进出的报文，无论请求还是响应，都必须经过Director Server, 请求报文由DS完成目标地址转换，响应报文由DS完成源地址转换

5） 在高负载应用场景中，DS很可能成为系统性能瓶颈。

6） 支持端口映射。

7） 内部RS可以使用任意支持集群服务的任意操作系统。
```

### 2，DR（Direct Routing）模式

DR模型是通过路由技术实现的负载均衡技术,而这种模型与NAT模型不同的地方是,负载均衡器通过改写用户请求报文中的MAC地址,将请求发送到RS, RS不用经过DIP而直接响应用户,这样就大大的减少负载均衡器的压力,DR模型也是用的最多的一种。

![lvs-dr](http://cdn.tianfeiyu.com/wp-content/uploads/2016/03/lvs-dr.png)

DR访问过程如下:

当客户端请求集群服务时，请求报文发送至 Director 的 VIP（RS的 VIP 不会响应 ARP 请求），Director 将客户端报文的源和目标 MAC 地址进行重新封装，将报文转发至 RS，RS 接收转发的报文。此时报文的源 IP 和目标 IP 都没有被修改，因此 RS 接受到的请求报文的目标 IP 地址为本机配置的 VIP，它将使用自己的 VIP 直接响应客户端。但是此模式下要求禁止RS响应对VIP的ARP广播请求，避免 VIP 被抢夺，要实现此功能有以下三种方法：

```go
（1） 在前端路由上实现静态MAC地址VIP的绑定；

前提：得有路由器的配置权限；

缺点：Directory故障转时，无法更新此绑定；

（2） arptables

前提：在各RS在安装arptables程序，并编写arptables规则

缺点：依赖于独特功能的应用程序

（3） 修改Linux内核参数

前提：RS必须是Linux；

缺点：适用性差；
```

Linux的工作特性：IP地址是属于主机，而非某特定网卡；也就是说，主机上所有的网卡都会向外通告,需要先配置参数，然后配置IP，因为只要IP地址配置完成则开始向外通告mac地址。为了使响应报文由配置有VIP的lo包装，使源地址为VIP，需要配置路由经过lo网卡的别名，最终由eth0发出。

```go
两个参数的取值含义：
arp_announce：定义通告模式

0： default， 只要主机接入网络，则自动通告所有为网卡mac地址

1： 尽力不通告非直接连入网络的网卡mac地址

2: 只通告直接进入网络的网卡mac地址

arp_ignore：定义收到arp请求时的响应模式

0： 只有arp 广播请求，马上响应，并且响应所有本机网卡的mac地址

1： 只响应，接受arp广播请求的网卡接口mac地址

2： 只响应，接受arp广播请求的网卡接口mac地址，并且需要请求广播与接口地址属于同一网段

3： 主机范围（Scope host）内生效的接口，不予响应，只响应全局生效与外网能通信的网卡接口

4-7： 保留位

8： 不响应一切arp广播请求

配置方法：

全部网卡

arp_ignore 1

arp_announce 2

同时再分别配置每个网卡,eth0和lo

arp_ignore 1

arp_annource 2
```

```go
DR模型特性：

（1）RS是可以使用公网地址，此时可以直接通过互联网连入，配置，监控RS服务器

（2）RS的网关不能指向DIP

（3）RS跟DS要在同一物理网络内，最好在一同一网段内

（4）请求报文经过Director但是响应报文不经过Director

（5）不支持端口映射

（6）RS可以使用，大多数的操作系统，至少要可以隐藏VIP
```

### 3，TUN 模式

TUN模型是通过IP隧道技术实现的,TUN模型跟DR模型有点类似,不同的地方是负载均衡器(Director Server)跟应用服务器(RS)通信的机制是通过IP隧道技术将用户的请求转发到某个RS,而 RS 也是直接响应用户的。

![lvs-tun](http://cdn.tianfeiyu.com/wp-content/uploads/2016/03/lvs-tun.png)

DR访问过程如下:

当请求到达 Director 后，Director 不修改请求报文的源 IP 和目标 IP 地址，而是使用 IP 隧道技术，使用 DIP 作为源 IP，RIP 作为目标 IP 再次封装此请求报文，转发至 RIP 的 RS 上，RS 解析报文后仍然使用 VIP 作为源地址响应客户端。

```go
TUN模型特性：

1）RIP，DIP，VIP都必须是公网地址

2）RS网关不会指向DIP

3）请求报文经过Director，但相应报文一定不经过Director

4）不支持端口映射

5）RS的OS必须得支持隧道功能
```

## 三、LVS调度算法

当 LVS 接受到一个客户端对集群服务的请求后，它需要进行决策将请求调度至某一台后端主机进行响应。LVS 的调度算法共有 10 种，按类别可以分为动态和静态两种类型。

**静态调度算法：静态调度算法调度时不会考虑后端服务器的负载状况和连接状态。**

**rr：round robin**，轮询，即简单在各主机间轮流调度

**wrr：weighted round robin**，加权轮询，根据各主机的权重进行轮询

**sh：source hash**，源地址哈希，对客户端地址进行哈希计算，保存在 Director 的哈希表中，在一段时间内，同一个客户端 IP 地址的请求会被调度至相同的 Realserver。sh 算法的目的是实现 session affinity（会话绑定），但是它也在一定程度上损害了负载均衡的效果。如果集群本身有 session sharing 机制或者没有 session 信息，那么不需要使用 sh 算法

**dh：destination hash**，和 sh 类似，dh 将请求的目标地址进行哈希，将相同 IP 的请求发送至同一主机，dh 机制的目的是，当 Realserver 为透明代理缓存服务器时，提高缓存的命中率。

**动态调度算法：动态调度算法在调度时，会根据后端 Realserver 的负载状态来决定调度选择，Realserver 的负载状态通常由活动链接（active），非活动链接（inactive）和权重来计算。**

**lc：least connted**，最少连接，LVS 根据 overhead = active*256 + inactive 计算服务器的负载状态，每次选择 overhead 最小的服务器

**wlc：weighted lc**，加权最少连接，LVS 根据 overhead = (active*256+inactive)/weight 来计算服务器负载，每次选择 overhead 最小的服务器，它是 LVS 的默认调度算法 。

**sed：shortest expected delay**，最短期望延迟，它不对 inactive 状态的连接进行计算，根据 overhead = (active+1)*256/weight 计算服务器负载，选择 overhead 最小的服务器进行调度

**nq：never queue**，当有空闲服务器时，直接调度至空闲服务器，当没有空闲服务器时，使用 SED 算法进行调度

**LBLC：locality based least connection**，基于本地的最少连接，相当于 dh + wlc，正常请求下使用 dh 算法进行调度，如果服务器超载，则使用 wlc 算法调度至其他服务器

**LBLCR：locality based least connection with replication**，基于本地的带复制功能的最少连接，与 LBLC 不同的是 LVS 将请求 IP 映射至一个服务池中，使用 dh 算法调度请求至对应的服务池中，使用 lc 算法选择服务池中的节点，当服务池中的所有节点超载，使用 lc 算法从所有后端 Realserver 中选择一个添加至服务吃中。

## 四，LVS缺陷

```go
不能检测后端服务器的健康状况，总是发送连接到后端。 

Session持久机制：

    1、session绑定：始终将同一个请求者的连接定向至同一个RS（第一次请求时仍由调度方法选择）；没有容错能力，有损均衡效果；

    2、session复制：在RS之间同步session，因此，每个RS持集群中所有的session；对于大规模集群环境不适用；

    3、session服务器：利用单独部署的服务器来统一管理session; 
```


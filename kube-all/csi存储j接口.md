#### 概述

kubernetes的设计初衷是支持可插拔架构，从而利于扩展kubernetes的功能。在此架构思想下，kubernetes提供了3个特定功能的接口，分别是容器网络接口CNI、容器运行时接口CRI和容器存储接口CSI。kubernetes通过调用这几个接口，来完成相应的功能。

下面我们来对容器存储接口CSI来做一下介绍与分析。

在本文中，会对CSI是什么、为什么要有CSI、CSI系统架构做一下介绍，然后对CSI所涉及的k8s对象与组件进行了简单的介绍，以及k8s对CSI存储进行相关操作的流程分析，存储相关操作包括了存储创建、存储扩容、存储挂载、解除存储挂载以及存储删除操作。

CSI是什么
CSI是Container Storage Interface（容器存储接口）的简写。

CSI的目的是定义行业标准“容器存储接口”，使存储供应商（SP）能够开发一个符合CSI标准的插件并使其可以在多个容器编排（CO）系统中工作。CO包括Cloud Foundry, Kubernetes, Mesos等。

kubernetes将通过CSI接口来跟第三方存储厂商进行通信，来操作存储，从而提供容器存储服务。

#### 为什么要有CSI

其实在没有CSI之前kubernetes就已经提供了强大的存储卷插件系统，但是这些插件系统实现是kubernetes代码的一部分，需要随kubernetes组件二进制文件一起发布，这样就会存在一些问题。

（1）如果第三方存储厂商发现有问题需要修复或者优化，即使修复后也不能单独发布，需要与kubernetes一起发布，对于k8s本身而言，不仅要考虑自身的正常迭代发版，还需要考虑到第三方存储厂商的迭代发版，这里就存在双方互相依赖、制约的问题，不利于双方快速迭代；
（2）另外第三方厂商的代码跟kubernetes代码耦合在一起，还会引起安全性、可靠性问题，还增加了kubernetes代码的复杂度以及后期的维护成本等等。

基于以上问题，kubernetes将存储体系抽象出了外部存储组件接口即CSI，kubernetes通过grpc接口与第三方存储厂商的存储卷插件系统进行通信。

这样一来，对于第三方存储厂商来说，既可以单独发布和部署自己的存储插件，进行正常迭代，而又无需接触kubernetes核心代码，降低了开发的复杂度。同时，对于kubernetes来说，这样不仅降低了自身的维护成本，还能为用户提供更多的存储选项。

#### CSI系统架构

这是一张k8s csi的系统架构图，图中所画的组件以及k8s对象，接下来会一一进行分析。

![image](https://user-images.githubusercontent.com/23715258/149048642-31c8758b-7e6f-4b67-a518-80ffcfb93a7b.png)

CSI相关组件一般采用容器化部署，减少环境依赖。

## 涉及k8s对象

### 1. PersistentVolume

持久存储卷，集群级别资源，代表了存储卷资源，记录了该存储卷资源的相关信息。

##### 回收策略

（1）retain：保留策略，当删除pvc的时候，保留pv与外部存储资源。

（2）delete：删除策略，当与pv绑定的pvc被删除的时候，会从k8s集群中删除pv对象，并执行外部存储资源的删除操作。

（3）resycle（已废弃）

##### pv状态迁移

available --> bound --> released

### 2. PersistentVolumeClaim

持久存储卷声明，namespace级别资源，代表了用户对于存储卷的使用需求声明。

示例：

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test
  namespace: test
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: csi-cephfs-sc
  volumeMode: Filesystem
```

###### pvc状态迁移

pending --> bound

### 3. StorageClass

定义了创建pv的模板信息，集群级别资源，用于动态创建pv。

示例：

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-rbd-sc
parameters:
  clusterID: ceph01
  imageFeatures: layering
  imageFormat: "2"
  mounter: rbd
  pool: kubernetes
provisioner: rbd.csi.ceph.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
```

### 4. VolumeAttachment

VolumeAttachment 记录了pv的相关挂载信息，如挂载到哪个node节点，由哪个volume plugin来挂载等。

AD Controller 创建一个 VolumeAttachment，而 External-attacher 则通过观察该 VolumeAttachment，根据其状态属性来进行存储的挂载和卸载操作。
示例：

```yaml
apiVersion: storage.k8s.io/v1
kind: VolumeAttachment
metadata:
  name: csi-123456
spec:
  attacher: cephfs.csi.ceph.com
  nodeName: 192.168.1.10
  source:
    persistentVolumeName: pvc-123456
status:
  attached: true
```

### 5. CSINode

CSINode 记录了csi plugin的相关信息（如nodeId、driverName、拓扑信息等）。

当Node Driver Registrar向kubelet注册一个csi plugin后，会创建（或更新）一个CSINode对象，记录csi plugin的相关信息。

示例：

```yaml
apiVersion: storage.k8s.io/v1
kind: CSINode
metadata:
  name: 192.168.1.10
spec:
  drivers:
  - name: cephfs.csi.ceph.com
    nodeID: 192.168.1.10
    topologyKeys: null
  - name: rbd.csi.ceph.com
    nodeID: 192.168.1.10
    topologyKeys: null
```

## 涉及组件与作用

![image](https://user-images.githubusercontent.com/23715258/149048831-77a191b3-ae98-47c0-aad8-10e811ade951.png)

下面来介绍下涉及的组件与作用。

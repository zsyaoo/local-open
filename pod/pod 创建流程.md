# Kubernetes中pod的创建流程

一般我们在创建pod的过程中都是，执行kubectl命令去apply对应的yaml文件，但是在执行这个操作的过程到pod被完成创建，k8s的组件都做了哪些操作呢？下面我们简要说说pod被创建的过程。

![image](https://user-images.githubusercontent.com/23715258/148713001-52ed9467-1168-47d6-b1ac-934797784da6.png)

1. 用户通过kubectl命名发起请求。
2. apiserver通过对应的kubeconfig进行认证，认证通过后将yaml中的po信息存到etcd。
3. Controller-Manager通过apiserver的watch接口发现了pod信息的更新，执行该资源所依赖的拓扑结构整合，整合后将对应的信息交给apiserver，apiserver写到etcd，此时pod已经可以被调度了。
4. Scheduler同样通过apiserver的watch接口更新到pod可以被调度，通过算法给pod分配节点，并将pod和对应节点绑定的信息交给apiserver，apiserver写到etcd，然后将pod交给kubelet。
5. kubelet收到pod后，调用CNI接口给pod创建pod网络，调用CRI接口去启动容器，调用CSI进行存储卷的挂载。
6. 网络，容器，存储创建完成后pod创建完成，等业务进程启动后，pod运行成功。

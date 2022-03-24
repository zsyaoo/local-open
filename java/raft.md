# Raft算法赏析

# 1 leader选举

- 1.1 刚开始所有server启动都是follower状态

  然后等待leader或者candidate的RPC请求、或者超时。

  上述3种情况处理如下：

  leader的AppendEntries RPC请求：更新term和leader信息，当前follower再重新重置到follower状态

  candidate的RequestVote RPC请求：为candidate进行投票，如果candidate的term比自己的大，则当前follower再重新重置到follower状态

  超时：转变为candidate，开始发起选举投票

- 1.2 candidate收集投票的过程

  candidate会为此次状态设置随机超时时间，一旦出现在当前term中大家都没有获取过半投票即split votes，超时时间短的更容易获得过半投票。

  candidate会向所有的server发送RequestVote RPC请求，请求参数见下面的官方图

  ![RequestVote RPC请求](https://static.oschina.net/uploads/img/201610/21112723_9uSU.png)

  上面对参数都说明的很清楚了，我们来重点说说图中所说的这段话

  ```
  If votedFor is null or candidateId, and candidate’s log is at
  least as up-to-date as receiver’s log, grant vote
  ```

  votedFor是server保存的投票对象，一个server在一个term内只能投一次票。如果此时已经投过票了，即votedFor就不为空，那么此时就可以直接拒绝当前的投票（当然还要检查votedFor是不是就是请求的candidate）。

  如果没有投过票：则对比candidate的log和当前server的log哪个更新，比较方式为谁的lastLog的term越大谁越新，如果term相同，谁的lastLog的index越大谁越新。

  candidate统计投票信息，如果过半同意了则认为自己当选了leader，转变成leader状态，如果没有过半，则等待是否有新的leader产生，如果有的话，则转变成follower状态，如果没有然后超时的话，则开启下一次的选举。

# 2 log复制

## 2.1 请求都交给leader

一旦leader选举成功，所有的client请求最终都会交给leader（如果client连接的是follower则follower转发给leader）

## 2.2 处理请求过程

- 2.2.1 client请求到达leader

  leader首先将该请求转化成entry，然后添加到自己的log中，得到该entry的index信息。entry中就包含了当前leader的term信息和在log中的index信息

- 2.2.2 leader复制上述entry到所有follower

  来看下官方给出的AppendEntries RPC请求

  ![AppendEntries RPC请求](https://static.oschina.net/uploads/img/201610/21153522_FdjP.png)

  ![各个状态拥有的参数](https://static.oschina.net/uploads/img/201610/21154029_X1G5.png)

  从上图可以看出对于每个follower，leader保持2个属性，一个就是nextIndex即leader要发给该follower的下一个entry的index，另一个就是matchIndex即follower发给leader的确认index。

  一个leader在刚开始的时候会初始化：

  ```
  nextIndex=leader的log的最大index+1
  matchIndex=0
  ```

  然后开始准备AppendEntries RPC请求的参数

  ```
  prevLogIndex=nextIndex-1
  prevLogTerm=从log中得到上述prevLogIndex对应的term
  ```

  然后开始准备entries数组信息

  ```
  从leader的log的prevLogIndex+1开始到lastLog，此时是空的
  ```

  然后把leader的commitIndex作为参数传给

  ```
  leaderCommit=commitIndex
  ```

  至此，所有参数准备完毕，发送RPC请求到所有的follower,follower再接收到这样的请求之后，处理如下：

  - 重置HeartbeatTimeout

  - 检查传过来的请求term和当前follower的term

    ```
    Reply false if term < currentTerm
    ```

  - 检查prevLogIndex和prevLogTerm和当前follower的对应index的log是否一致，

    ```
    Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
    ```

    这里可能就是不一致的，因为初始prevLogIndex和prevLogTerm是leader上log的lastLog，不一致的话返回false，同时将该follower上log的lastIndex传送给leader

  - leader接收到上述false之后，会记录该follower的上述lastIndex

    ```
    macthIndex=上述lastIndex
    nextIndex=上述lastIndex+1
    ```

    然后leader会从新按照上述规则，发送新的prevLogIndex、prevLogTerm、和entries数组

  - follower检查prevLogIndex和prevLogTerm和对应index的log是否一致（目前一致了）

  - 然后follower就开始将entries中的数据全部覆盖到本地对应的index上，如果没有则算是添加如果有则算是更新，也就是说和leader的保持一致

  - 最后follower将最后复制的index发给leader,同时返回ok，leader会像上述一样来更新follower的macthIndex

- 2.2.3 leader统计过半复制的entries

  leader一旦发现有些entries已经被过半的follower复制了，则就将该entry提交，将commitIndex提升至该entry的index。（这里是按照entry的index先后顺序提交的），具体的实现可以通过follower发送过来macthIndex来判定是否过半了

  一旦可以提交了，leader就将该entry应用到状态机中，然后给客户端回复OK

  然后在下一次heartBeat心跳中，将commitIndex就传给了所有的follower，对应的follower就可以将commitIndex以及之前的entry应用到各自的状态机中了

# 3 安全

## 3.1 选举约束

对于上述leader选举有个重点强调的地方就是

```
被选举出来的leader必须要包含所有已经比提交的entries
```

如leader针对复制过半的entry提交了，但是某些follower可能还没有这些entry，当leader挂了，该follower如果被选举成leader的时候，就可能会覆盖掉了上述的entry了，造成不一致的问题，所以新选出来的leader必须要满足上述约束

目前对于上述约束的简单实现就是：

```
只要当前server的log比半数server的log都新就可以，这里的新就是上述说的：

谁的lastLog的term越大谁越新，如果term相同，谁的lastLog的index越大谁越新
```

但是正是这个实现并不能完全实现约束，才会产生下面的另外一个问题，一会会详细案例来说明这个问题

## 3.2 当前term的leader是否能够直接提交之前term的entries

raft给出的答案是：

```
当前term的leader不能“直接”提交之前term的entries
```

也就是可以间接的方式来提交。我们来看下raft给出不能直接提交的案例

![不能直接提交之前term的entries的案例](https://static.oschina.net/uploads/img/201610/21164723_v2Rh.png)

最上面一排数字表示的是index，s1-s5表示的是server服务器，a-e表示的是不同的场景，方框里面的数字表示的是term

详细解释如下：

- a场景：s1是leader,此时处于term2，并且将index为2的entry复制到s2上

- b场景：s1挂了，s5当选为leader，处于term3,s5在index为2的位置上接收到了新的entry

- c场景：s5挂了，s1当选为leader，处于term4，s1将index为2,term为2的entry复制到了s3上，此时已经满足过半数了

  重点就在这里：此时处于term4,但是之前处于term2的entry达到过半数了，s1是提交该entry呢还是不提交呢？

  假如s1提交的话，则index为2，term为2的entry就被应用到状态机中了，是不可改变了，此时s1如果挂了，来到term5，s5是可以被选为leader的，因为按照之前的log比对策略来说，s5的最后一个log的term是3比s2 s3 s4的最后一个log的term都大。一旦s5被选举为leader，即d场景，s5会复制index为2,term为3的entry到上述机器上，这时候就会造成之前s1已经提交的index为2的位置被重新覆盖，因此违背了一致性。

  假如s1不提交，而是等到term4中有过半的entry了，然后再将之前的term的entry一起提交（这就是所谓的间接提交，即使满足过半，但是必须要等到当前term中有过半的entry才能跟着一起提交），即处于e场景，s1此时挂的话，s5就不能被选为leader了，因为s2 s3的最后一个log的term为4比s5的3大，所以s5获取不到投票，进而s5就不可能去覆盖上述的提交

这里再对日志覆盖问题进行详细阐述

日志覆盖包含2种情况：

- commitIndex之后的log覆盖:是允许的，如leader发送AppendEntries RPC请求给follower，follower都会进行覆盖纠正，以保持和leader一致。
- commitIndex及其之前的log覆盖：是禁止的，因为这些已经被应用到状态机中了，一旦再覆盖就出现了不一致性。而上述案例中的覆盖就是指这种情况的覆盖。

从这个案例中我们得到的一个新约束就是：

```
当前term的leader不能“直接”提交之前term的entries
必须要等到当前term有entry过半了，才顺便一起将之前term的entries进行提交
```

所以raft靠着这2个约束来进一步保证一致性问题。

再来仔细分析这个案例，其问题就是出在：上述leader选举上，s1如果在c场景下将index为2、term为2的entry提交了，此时s5也就不包含所有的commitLog了，但是s5按照log最新的比较方法还是能当选leader，那就是说log最新的比较方法并不能保证3.1中的选举约束即

```
被选举出来的leader必须要包含所有已经比提交的entries
```

所以可以理解为：正是由于上述选举约束实现上的缺陷才导致又加了这么一个不能直接提交之前term的entries的约束。

## 3.3 安全性论证

Leader Completeness： 如果一个entry被提交了，那么在之后的leader中，必然存在该entry。

经过上述2个约束，就能得出Leader Completeness结论。

正是由于上述“不能直接提交之前term的entries”的约束，所以**任何一个entry的提交必然存在当前term下的entry的提交**。那么此时所有的server中有过半的server都含有当前term(也是当前最大的term)的entry，假设serverA将来会成为leader，此时serverA的lastlog的term必然是不大于当前term的，它要想成为leader，即和其他server pk 谁的log最新，必然是需要满足log的index比他们大的，所以必然含有已提交的entry。

# 4 其他注意点

## 4.1 client端

在client看来：

如果client发送一个请求，leader返回ok响应，那么client认为这次请求成功执行了，那么这个请求就需要被真实的落地，不能丢。

如果leader没有返回ok，那么client可以认为这次请求没有成功执行，之后可以通过重试方式来继续请求。

所以对leader来说：

一旦你给客户端回复OK的话，然后挂了，那么这个请求对应的entry必须要保证被应用到状态机，即需要别的leader来继续完成这个应用到状态机。

一旦leader在给客户端答复之前挂了，那么这个请求对应的entry就不能被应用到状态机了，如果被应用到状态机就造成客户端认为执行失败，但是服务器端缺持久化了这个请求结果，这就有点不一致了。

这个原则同消息队列也是一致的。再来说说什么叫消息队列的消息丢失（很多人还没真正搞明白这个问题）：client向服务器端发送消息，服务器端回复OK了，之后因为服务器端自己的内部机制的原因导致该消息丢失了，这种情况才叫消息队列的消息丢失。如果服务器端没有给你回复OK，那么这种情况就不属于消息队列丢失消息的范畴。

再来看看raft是否能满足这个原则：

leader在某个entry被过半复制了，认为可以提交了，就应用到状态机了，然后向客户端回复OK，之后leader挂了，是可以保证该entry在之后的leader中是存在的

leader在某个entry被过半复制了，然后就挂了，即没有向客户端回复OK，raft的机制下，后来的leader是可能会包含该entry并提交的，或可能直接就覆盖掉了该entry。如果是前者，则该entry是被应用到了状态机中，那么此时就出现一个问题：client没有收到OK回复，但是服务器端竟然可以成功保存了

为了掩盖这种情况，就需要在客户端做一次手脚，即客户端对那么没有回复OK的都要进行重试，客户端的请求都带着一个唯一的请求id，重试的时候也是拿着之前的请求id去重试的

服务器端发现该请求id已经存在提交log中了，那么直接回复OK，如果不在的话，那么再执行一次该请求。

## 4.2 follower挂了

follower挂了，只要leader还满足过半条件就，一切正常。他们挂了又恢复之后，leader是会不断进行重试的，该follower仍然是能恢复正常的

follower在接收AppendEntries RPC的时候是幂等操作

## 4.3 集群成员调整

目前这一块稍后再说

最后就再说下raft的java版实现，可以看下[copycat](https://www.oschina.net/action/GoToLink?url=https%3A%2F%2Fgithub.com%2Fatomix%2Fcopycat)


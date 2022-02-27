创建线程有几种方式？这个问题的答案应该是可以脱口而出的吧

- 继承 Thread 类
- 实现 Runnable 接口

但这两种方式创建的线程是属于”三无产品“：

- 没有参数
- 没有返回值
- 没办法抛出异常

```
class MyThread implements Runnable{
   @Override
   public void run() {
      log.info("my thread");
   }
}
```

Runnable 接口是 JDK1.0 的核心产物

```
 /**
 * @since   JDK1.0
 */
@FunctionalInterface
public interface Runnable {
    public abstract void run();
}
```

用着 “三无产品” 总是有一些弊端，其中没办法拿到返回值是最让人不能忍的，于是 Callable 就诞生了

## Callable

又是 Doug Lea 大师，又是 Java 1.5 这个神奇的版本

```
 /**
 * @see Executor
 * @since 1.5
 * @author Doug Lea
 * @param <V> the result type of method {@code call}
 */
@FunctionalInterface
public interface Callable<V> {
    
    V call() throws Exception;
}
```

Callable 是一个泛型接口，里面只有一个 `call()` 方法，**该方法可以返回泛型值 V** ，使用起来就像这样：

```
Callable<String> callable = () -> {
    // Perform some computation
    Thread.sleep(2000);
    return "Return some result";
};
```

二者都是函数式接口，里面都仅有一个方法，使用上又是如此相似，除了有无返回值，Runnable 与 Callable 就点差别吗？

## Runnable VS Callable

两个接口都是用于多线程执行任务的，但他们还是有很明显的差别的

### 执行机制

先从执行机制上来看，Runnable 你太清楚了，它既可以用在 Thread 类中，也可以用在 *ExecutorService* 类中配合线程池的使用；**Bu～～～～t， Callable 只能在 \*ExecutorService\* 中使用**，你翻遍 Thread 类，也找不到Callable 的身影

![image](https://user-images.githubusercontent.com/23715258/155870151-391ee47a-05cd-4a62-a387-94cefeca7349.png)

### 异常处理

Runnable 接口中的 run 方法签名上没有 **throws** ，自然也就没办法向上传播受检异常；而 Callable 的 call() 方法签名却有 **throws**，所以它可以处理受检异常；

所以归纳起来看主要有这几处不同点：

![image](https://user-images.githubusercontent.com/23715258/155870170-b97a53eb-1213-46f9-bf60-9d67297c628d.png)

整体差别虽然不大，但是这点差别，却具有重大意义

返回值和处理异常很好理解，另外，在实际工作中，我们通常要使用线程池来管理线程（*原因已经在 [为什么要使用线程池?](https://dayarch.top/p/why-we-need-to-use-threadpool.html) 中明确说明*），所以我们就来看看 ExecutorService 中是如何使用二者的

## ExecutorService

先来看一下 ExecutorService 类图

![image](https://user-images.githubusercontent.com/23715258/155870190-8c0d6744-402c-40d3-ac10-ce09292e3289.png)

我将上图标记的方法单独放在此处

```
void execute(Runnable command);

<T> Future<T> submit(Callable<T> task);
<T> Future<T> submit(Runnable task, T result);
Future<?> submit(Runnable task);
```

可以看到，使用ExecutorService 的 `execute()` 方法依旧得不到返回值，而 `submit()` 方法清一色的返回 `Future` 类型的返回值

- Future 到底是什么呢？
- 怎么通过它获取返回值呢？

我们带着这些疑问一点点来看

## Future

Future 又是一个接口，里面只有五个方法：

![image](https://user-images.githubusercontent.com/23715258/155870226-c83bc562-0dcf-460a-b6e9-32ccdf6fc2e6.png)

从方法名称上相信你已经能看出这些方法的作用

```
// 取消任务
boolean cancel(boolean mayInterruptIfRunning);

// 获取任务执行结果
V get() throws InterruptedException, ExecutionException;

// 获取任务执行结果，带有超时时间限制
V get(long timeout, TimeUnit unit) throws InterruptedException, ExecutionException,  TimeoutException;

// 判断任务是否已经取消
boolean isCancelled();

// 判断任务是否已经结束
boolean isDone();
```

铺垫了这么多，看到这你也许有些乱了，咱们赶紧看一个例子，演示一下几个方法的作用

```
@Slf4j
public class FutureAndCallableExample {

   public static void main(String[] args) throws InterruptedException, ExecutionException {
      ExecutorService executorService = Executors.newSingleThreadExecutor();

      // 使用 Callable ，可以获取返回值
      Callable<String> callable = () -> {
         log.info("进入 Callable 的 call 方法");
         // 模拟子线程任务，在此睡眠 2s，
         // 小细节：由于 call 方法会抛出 Exception，这里不用像使用 Runnable 的run 方法那样 try/catch 了
         Thread.sleep(5000);
         return "Hello from Callable";
      };

      log.info("提交 Callable 到线程池");
      Future<String> future = executorService.submit(callable);

      log.info("主线程继续执行");

      log.info("主线程等待获取 Future 结果");
      // Future.get() blocks until the result is available
      String result = future.get();
      log.info("主线程获取到 Future 结果: {}", result);

      executorService.shutdown();
   }
}
```

程序运行结果如下：

![image](https://user-images.githubusercontent.com/23715258/155870240-98a92f43-5fe6-4bdd-8f86-ff04aa733a79.png)

如果你运行上述示例代码，主线程调用 future.get() 方法会阻塞自己，直到子任务完成。我们也可以使用 Future 方法提供的 `isDone` 方法，它可以用来检查 task 是否已经完成了，我们将上面程序做点小修改：

```
// 如果子线程没有结束，则睡眠 1s 重新检查
while(!future.isDone()) {
   System.out.println("Task is still not done...");
   Thread.sleep(1000);
}
```

来看运行结果：

![image](https://user-images.githubusercontent.com/23715258/155870268-b587cb37-82ca-4524-af9a-2a61ab8e534c.png)

如果子程序运行时间过长，或者其他原因，我们想 cancel 子程序的运行，则我们可以使用 Future 提供的 cancel 方法，继续对程序做一些修改

```
while(!future.isDone()) {
   System.out.println("子线程任务还没有结束...");
   Thread.sleep(1000);

   double elapsedTimeInSec = (System.nanoTime() - startTime)/1000000000.0;

 	 // 如果程序运行时间大于 1s，则取消子线程的运行
   if(elapsedTimeInSec > 1) {
      future.cancel(true);
   }
}
```

来看运行结果：

![image](https://user-images.githubusercontent.com/23715258/155870279-3a56859a-9288-421a-8f7e-55445e852512.png)

为什么调用 cancel 方法程序会出现 CancellationException 呢？ 是因为调用 get() 方法时，明确说明了：

> 调用 get() 方法时，如果计算结果被取消了，则抛出 CancellationException （具体原因，你会在下面的源码分析中看到）

![image](https://user-images.githubusercontent.com/23715258/155870290-768cebb0-5d53-4ebe-aa7c-03b229f04ba2.png)

有异常不处理是非常不专业的，所以我们需要进一步修改程序，以更友好的方式处理异常

```
// 通过 isCancelled 方法判断程序是否被取消，如果被取消，则打印日志，如果没被取消，则正常调用 get() 方法
if (!future.isCancelled()){
   log.info("子线程任务已完成");
   String result = future.get();
   log.info("主线程获取到 Future 结果: {}", result);
}else {
   log.warn("子线程任务被取消");
}
```

查看程序运行结果：

![image](https://user-images.githubusercontent.com/23715258/155870310-e85c44e0-1ff6-42cb-8fd8-58182ab082a4.png)

相信到这里你已经对 `Future` 的几个方法有了基本的使用印象，但 `Future` 是接口，其实使用 `ExecutorService.submit()` 方法返回的一直都是 `Future` 的实现类 `FutureTask`

![image](https://user-images.githubusercontent.com/23715258/155870322-15d60909-f87d-43f9-9c0d-b834943fb0a0.png)

接下来我们就进入这个核心实现类一探究竟

## FutureTask

同样先来看类结构

![image](https://user-images.githubusercontent.com/23715258/155870337-36dfd866-f808-4e6c-93d9-1aed28e084cc.png)

```
public interface RunnableFuture<V> extends Runnable, Future<V> {
    void run();
}
```

很神奇的一个接口，`FutureTask` 实现了 `RunnableFuture` 接口，而 `RunnableFuture` 接口又分别实现了 `Runnable` 和 `Future` 接口，所以可以推断出 `FutureTask` 具有这两种接口的特性：

- 有 `Runnable` 特性，所以可以用在 `ExecutorService` 中配合线程池使用
- 有 `Future` 特性，所以可以从中获取到执行结果

### FutureTask源码分析

如果你完整的看过 AQS 相关分析的文章，你也许会发现，阅读 Java 并发工具类源码，我们无非就是要关注以下这三点：

```
- 状态 （代码逻辑的主要控制）
- 队列 （等待排队队列）
- CAS （安全的set 值）
```

> 脑海中牢记这三点，咱们开始看 FutureTask 源码，看一下它是如何围绕这三点实现相应的逻辑的

文章开头已经提到，实现 Runnable 接口形式创建的线程并不能获取到返回值，而实现 Callable 的才可以，所以 FutureTask 想要获取返回值，必定是和 Callable 有联系的，这个推断一点都没错，从构造方法中就可以看出来：

```
public FutureTask(Callable<V> callable) {
    if (callable == null)
        throw new NullPointerException();
    this.callable = callable;
    this.state = NEW;       // ensure visibility of callable
}
```

即便在 FutureTask 构造方法中传入的是 Runnable 形式的线程，该构造方法也会通过 `Executors.callable` 工厂方法将其转换为 Callable 类型：

```
public FutureTask(Runnable runnable, V result) {
    this.callable = Executors.callable(runnable, result);
    this.state = NEW;       // ensure visibility of callable
}
```

但是 FutureTask 实现的是 Runnable 接口，也就是只能重写 run() 方法，run() 方法又没有返回值，那问题来了：

- FutureTask 是怎样在 run() 方法中获取返回值的？
- 它将返回值放到哪里了？
- get() 方法又是怎样拿到这个返回值的呢？

我们来看一下 run() 方法（关键代码都已标记注释）

```
public void run() {
  	// 如果状态不是 NEW，说明任务已经执行过或者已经被取消，直接返回
  	// 如果状态是 NEW，则尝试把执行线程保存在 runnerOffset（runner字段），如果赋值失败，则直接返回
    if (state != NEW ||
        !UNSAFE.compareAndSwapObject(this, runnerOffset,
                                     null, Thread.currentThread()))
        return;
    try {
      	// 获取构造函数传入的 Callable 值
        Callable<V> c = callable;
        if (c != null && state == NEW) {
            V result;
            boolean ran;
            try {
              	// 正常调用 Callable 的 call 方法就可以获取到返回值
                result = c.call();
                ran = true;
            } catch (Throwable ex) {
                result = null;
                ran = false;
              	// 保存 call 方法抛出的异常
                setException(ex);
            }
            if (ran)
              	// 保存 call 方法的执行结果
                set(result);
        }
    } finally {        
        runner = null;       
        int s = state;
      	// 如果任务被中断，则执行中断处理
        if (s >= INTERRUPTING)
            handlePossibleCancellationInterrupt(s);
    }
}
```


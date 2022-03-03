#kafka配置，更多配置请参考：KafkaProperties
spring.kafka:
  #公共参数，其他的timeout.ms, request.timeout.ms, metadata.fetch.timeout.ms保持默认值
  properties:
    #这个参数指定producer在发送批量消息前等待的时间，当设置此参数后，即便没有达到批量消息的指定大小(batch-size)，到达时间后生产者也会发送批量消息到broker。默认情况下，生产者的发送消息线程只要空闲了就会发送消息，即便只有一条消息。设置这个参数后，发送线程会等待一定的时间，这样可以批量发送消息增加吞吐量，但同时也会增加延迟。
    linger.ms: 50 #默认值：0毫秒，当消息发送比较频繁时，增加一些延迟可增加吞吐量和性能。
    #这个参数指定producer在一个TCP connection可同时发送多少条消息到broker并且等待broker响应，设置此参数较高的值可以提高吞吐量，但同时也会增加内存消耗。另外，如果设置过高反而会降低吞吐量，因为批量消息效率降低。设置为1，可以保证发送到broker的顺序和调用send方法顺序一致，即便出现失败重试的情况也是如此。
    #注意：当前消息符合at-least-once，自kafka1.0.0以后，为保证消息有序以及exactly once，这个配置可适当调大为5。
    max.in.flight.requests.per.connection: 1 #默认值：5，设置为1即表示producer在connection上发送一条消息，至少要等到这条消息被broker确认收到才继续发送下一条，因此是有序的。

  #生产者的配置，可参考org.apache.kafka.clients.producer.ProducerConfig
  producer:
    #这个参数可以是任意字符串，它是broker用来识别消息是来自哪个客户端的。在broker进行打印日志、衡量指标或者配额限制时会用到。
    clientId: ${spring.application.name} #方便kafkaserver打印日志定位请求来源
    bootstrap-servers: 127.0.0.1:8080 #kafka服务器地址，多个以逗号隔开
    #acks=0：生产者把消息发送到broker即认为成功，不等待broker的处理结果。这种方式的吞吐最高，但也是最容易丢失消息的。
    #acks=1：生产者会在该分区的leader写入消息并返回成功后，认为消息发送成功。如果群首写入消息失败，生产者会收到错误响应并进行重试。这种方式能够一定程度避免消息丢失，但如果leader宕机时该消息没有复制到其他副本，那么该消息还是会丢失。另外，如果我们使用同步方式来发送，延迟会比前一种方式大大增加（至少增加一个网络往返时间）；如果使用异步方式，应用感知不到延迟，吞吐量则会受异步正在发送中的数量限制。
    #acks=all：生产者会等待所有副本成功写入该消息，这种方式是最安全的，能够保证消息不丢失，但是延迟也是最大的。
    #如果是发送日志之类的，允许部分丢失，可指定acks=0，如果想不丢失消息，可配置为all，但需密切关注性能和吞吐量。
    acks: all #默认值：1
    #当生产者发送消息收到一个可恢复异常时，会进行重试，这个参数指定了重试的次数。在实际情况中，这个参数需要结合retry.backoff.ms（重试等待间隔）来使用，建议总的重试时间比集群重新选举leader的时间长，这样可以避免生产者过早结束重试导致失败。
    #另外需注意，当开启重试时，若未设置max.in.flight.requests.per.connection=1，则可能出现发往同一个分区的两批消息的顺序出错，比如，第一批发送失败了，第二批成功了，然后第一批重试成功了，此时两者的顺序就颠倒了。
    retries: 2  #发送失败时重试多少次，0=禁用重试（默认值）
    #默认情况下消息是不压缩的，此参数可指定采用何种算法压缩消息，可取值：none,snappy,gzip,lz4。snappy压缩算法由Google研发，这种算法在性能和压缩比取得比较好的平衡；相比之下，gzip消耗更多的CPU资源，但是压缩效果也是最好的。通过使用压缩，我们可以节省网络带宽和Kafka存储成本。
    compressionType: "none" #如果不开启压缩，可设置为none（默认值），比较大的消息可开启。
    #当多条消息发送到一个分区时，Producer会进行批量发送，这个参数指定了批量消息大小的上限（以字节为单位）。当批量消息达到这个大小时，Producer会一起发送到broker；但即使没有达到这个大小，生产者也会有定时机制来发送消息，避免消息延迟过大。
    batch-size: 16384 #默认16K，值越小延迟越低，但是吞吐量和性能会降低。0=禁用批量发送
    #这个参数设置Producer暂存待发送消息的缓冲区内存的大小，如果应用调用send方法的速度大于Producer发送的速度，那么调用会阻塞一定（max.block.ms）时间后抛出异常。
    buffer-memory: 33554432 #缓冲区默认大小32M
  #消费者的配置，可参考：org.apache.kafka.clients.consumer.ConsumerConfig
  consumer:
    #这个参数可以为任意值，用来指明消息从哪个客户端发出，一般会在打印日志、衡量指标、分配配额时使用。
    #暂不用提供clientId，2.x版本可放出来，1.x有多个topic且concurrency>1会出现JMX注册时异常
    #clientId: ${spring.application.name} #方便kafkaserver打印日志定位请求来源
    # 签中kafka集群
    bootstrap-servers: 127.0.0.1:8080 #kafka服务器地址，多个以逗号隔开
    #这个参数指定了当消费者第一次读取分区或者无offset时拉取那个位置的消息，可以取值为latest（从最新的消息开始消费）,earliest（从最老的消息开始消费）,none（如果无offset就抛出异常）
    autoOffsetReset: latest #默认值：latest
    #这个参数指定了消费者是否自动提交消费位移，默认为true。如果需要减少重复消费或者数据丢失，你可以设置为false，然后手动提交。如果为true，你可能需要关注自动提交的时间间隔，该间隔由auto.commit.interval.ms设置。
    enable-auto-commit: false
    #周期性自动提交的间隔，单位毫秒
    auto-commit-interval: 2000 #默认值：5000
    #这个参数允许消费者指定从broker读取消息时最小的Payload的字节数。当消费者从broker读取消息时，如果数据字节数小于这个阈值，broker会等待直到有足够的数据，然后才返回给消费者。对于写入量不高的主题来说，这个参数可以减少broker和消费者的压力，因为减少了往返的时间。而对于有大量消费者的主题来说，则可以明显减轻broker压力。
    fetchMinSize: 1 #默认值： 1
    #上面的fetch.min.bytes参数指定了消费者读取的最小数据量，而这个参数则指定了消费者读取时最长等待时间，从而避免长时间阻塞。这个参数默认为500ms。
    fetchMaxWait: 500 #默认值：500毫秒
    #这个参数控制一个poll()调用返回的记录数，即consumer每次批量拉多少条数据。
    maxPollRecords: 500 #默认值：500
  listener:
    #创建多少个consumer，值必须小于等于Kafk Topic的分区数。
    ack-mode: MANUAL_IMMEDIATE
    concurrency: 1  #推荐设置为topic的分区数

#  kubebuilder开发cronjob控制器

## operator介绍

Kubernetes 为自动化而生。无需任何修改，你即可以从 Kubernetes 核心中获得许多内置的自动化功能。 你可以使用 Kubernetes 自动化部署和运行工作负载， *甚至* 可以自动化 Kubernetes 自身。

Kubernetes 的 [Operator 模式](https://kubernetes.io/zh/docs/concepts/extend-kubernetes/operator/)概念 使你无需修改 Kubernetes 自身的代码，通过把定制控制器关联到一个以上的定制资源上，即可以扩展集群的行为。 Operator 是 Kubernetes API 的客户端，充当 [定制资源](https://kubernetes.io/zh/docs/concepts/extend-kubernetes/api-extension/custom-resources/) 的控制器。

开发一个operator，我们需要定义CRD，生成controller依赖模块，编写controller业务逻辑，为了方便开发，[kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)帮助我们自动解决这些问题，使得我们专注业务逻辑。



## 初始化项目

在初始化项目之前，需要提前安装kubebuilder，直接在[社区](https://github.com/kubernetes-sigs/kubebuilder/releases)选择下载，kubebuilder是一个二进制程序。

```bash
mv kubebuilder_linux_amd64 kubebuilder && mv kubebuilder /usr/bin && chmod +x kubebuilder 
kubebuilder version
Version: main.version{KubeBuilderVersion:"3.2.0", KubernetesVendor:"1.22.1", GitCommit:"b7a730c84495122a14a0faff95e9e9615fffbfc5", BuildDate:"2021-10-29T18:32:16Z", GoOs:"linux", GoArch:"amd64"}
```

接下来使用kubebuilder创建项目

```bash
# 创建项目目录
$ mkdir -p cronjob && cd cronjob
$ export GO111MODULE=on  # 使用gomodules包管理工具
$ export GOPROXY="https://goproxy.cn" 
# 初始化项目--domain 表示域，--owner表示作者，--repo表示项目mod
$ kubebuilder init --domain sfeng.io --owner sfeng --repo github.com/sfeng/cronjob
# 初始化成功后，查看项目目录结构
$ tree .
.
├── config
│   ├── default
│   │   ├── kustomization.yaml
│   │   ├── manager_auth_proxy_patch.yaml
│   │   └── manager_config_patch.yaml
│   ├── manager
│   │   ├── controller_manager_config.yaml
│   │   ├── kustomization.yaml
│   │   └── manager.yaml
│   ├── prometheus
│   │   ├── kustomization.yaml
│   │   └── monitor.yaml
│   └── rbac
│       ├── auth_proxy_client_clusterrole.yaml
│       ├── auth_proxy_role_binding.yaml
│       ├── auth_proxy_role.yaml
│       ├── auth_proxy_service.yaml
│       ├── kustomization.yaml
│       ├── leader_election_role_binding.yaml
│       ├── leader_election_role.yaml
│       ├── role_binding.yaml
│       └── service_account.yaml
├── Dockerfile
├── go.mod
├── go.sum
├── hack
│   └── boilerplate.go.txt
├── main.go
├── Makefile
└── PROJECT

6 directories, 24 files
```

## 创建CronJob API

operator的作用就是我们自定义k8s资源，然后开发controller控制资源达到期望状态，那么自定义资源就是一个gvk(Group Version Kind)。

```bash
$ kubebuilder create api --group batch --version v1 --kind CronJob
Create Resource [y/n]
y
Create Controller [y/n]
y
Writing kustomize manifests for you to edit...
Writing scaffold for you to edit...
api/v1/cronjob_types.go
controllers/cronjob_controller.go
Update dependencies:
$ go mod tidy
Running make:
$ make generate
go: creating new go.mod: module tmp
Downloading sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0
go: downloading sigs.k8s.io/controller-tools v0.7.0
go: downloading golang.org/x/tools v0.1.5
go: downloading github.com/spf13/cobra v1.2.1
go: downloading github.com/fatih/color v1.12.0
go: downloading k8s.io/api v0.22.2
go: downloading k8s.io/apimachinery v0.22.2
go: downloading k8s.io/apiextensions-apiserver v0.22.2
go: downloading github.com/gobuffalo/flect v0.2.3
go: downloading github.com/inconshreveable/mousetrap v1.0.0
go: downloading github.com/mattn/go-colorable v0.1.8
go: downloading github.com/mattn/go-isatty v0.0.12
go: downloading k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
go: downloading github.com/google/go-cmp v0.5.6
go: downloading golang.org/x/sys v0.0.0-20210616094352-59db8d763f22
go: downloading golang.org/x/mod v0.4.2
go get: added sigs.k8s.io/controller-tools v0.7.0
/home/sfeng/go/src/cronjob/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
Next: implement your new API and generate the manifests (e.g. CRDs,CRs) with:
$ make manifests
```

发现目录结构多了api，controller等目录和文件，api目录下为我们自定义api的结构体

```bash
$ tree .
.
├── api
│   └── v1
│       ├── cronjob_types.go
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go
├── bin
│   └── controller-gen
├── config
│   ├── crd
│   │   ├── kustomization.yaml
│   │   ├── kustomizeconfig.yaml
│   │   └── patches
│   │       ├── cainjection_in_cronjobs.yaml
│   │       └── webhook_in_cronjobs.yaml
│   ├── default
│   │   ├── kustomization.yaml
│   │   ├── manager_auth_proxy_patch.yaml
│   │   └── manager_config_patch.yaml
│   ├── manager
│   │   ├── controller_manager_config.yaml
│   │   ├── kustomization.yaml
│   │   └── manager.yaml
│   ├── prometheus
│   │   ├── kustomization.yaml
│   │   └── monitor.yaml
│   ├── rbac
│   │   ├── auth_proxy_client_clusterrole.yaml
│   │   ├── auth_proxy_role_binding.yaml
│   │   ├── auth_proxy_role.yaml
│   │   ├── auth_proxy_service.yaml
│   │   ├── cronjob_editor_role.yaml
│   │   ├── cronjob_viewer_role.yaml
│   │   ├── kustomization.yaml
│   │   ├── leader_election_role_binding.yaml
│   │   ├── leader_election_role.yaml
│   │   ├── role_binding.yaml
│   │   └── service_account.yaml
│   └── samples
│       └── batch_v1_cronjob.yaml
├── controllers
│   ├── cronjob_controller.go
│   └── suite_test.go
├── Dockerfile
├── go.mod
├── go.sum
├── hack
│   └── boilerplate.go.txt
├── main.go
├── Makefile
└── PROJECT

13 directories, 37 files
```

## 设计API

cronjob的功能应该是定时执行job，所以cronjob的spec应该包含

- 时间表
- 要运行的job模板

当然CronJob还需要一些额外的东西，使得它更加易用

- 一个已经启动的Job的超时时间(如果该Job执行超时，那么我们会将下次调度的时候重新执行该Job）
- 如果多个 Job 同时运行，该怎么办（我们要等待吗？还是停止旧的 Job ？）
- 暂停 CronJob 运行的方法，以防出现问题。
- 对旧 Job 历史的限制

我们将使用几个标记（`// +comment`）来指定额外的元数据。在生成 CRD 清单时，[controller-tools](https://github.com/kubernetes-sigs/controller-tools) 将使用这些数据。我们稍后将看到，controller-tools 也将使用 GoDoc 来生成字段的描述。

$ vim *[project/api/v1/cronjob_types.go](https://sigs.k8s.io/kubebuilder/docs/book/src/cronjob-tutorial/testdata/project/api/v1/cronjob_types.go)*

```go
package v1

import (
    batchv1beta1 "k8s.io/api/batch/v1beta1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
```

```go
// CronJobSpec defines the desired state of CronJob
type CronJobSpec struct {
    // +kubebuilder:validation:MinLength=0

    // The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
    Schedule string `json:"schedule"`

    // +kubebuilder:validation:Minimum=0

    // Optional deadline in seconds for starting the job if it misses scheduled
    // time for any reason.  Missed jobs executions will be counted as failed ones.
    // +optional
    StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

    // Specifies how to treat concurrent executions of a Job.
    // Valid values are:
    // - "Allow" (default): allows CronJobs to run concurrently;
    // - "Forbid": forbids concurrent runs, skipping next run if previous run hasn't finished yet;
    // - "Replace": cancels currently running job and replaces it with a new one
    // +optional
    ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

    // This flag tells the controller to suspend subsequent executions, it does
    // not apply to already started executions.  Defaults to false.
    // +optional
    Suspend *bool `json:"suspend,omitempty"`

    // Specifies the job that will be created when executing a CronJob.
    JobTemplate batchv1beta1.JobTemplateSpec `json:"jobTemplate"`

    // +kubebuilder:validation:Minimum=0

    // The number of successful finished jobs to retain.
    // This is a pointer to distinguish between explicit zero and not specified.
    // +optional
    SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`

    // +kubebuilder:validation:Minimum=0

    // The number of failed finished jobs to retain.
    // This is a pointer to distinguish between explicit zero and not specified.
    // +optional
    FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`
}
```

我们定义了一个自定义类型来保存我们的并发策略。实际上，它的底层类型是 string，但该类型给出了额外的文档，并允许我们在类型上附加验证，而不是在字段上验证，使验证逻辑更容易复用。

```go
// ConcurrencyPolicy describes how the job will be handled.
// Only one of the following concurrent policies may be specified.
// If none of the following policies is specified, the default one
// is AllowConcurrent.
// +kubebuilder:validation:Enum=Allow;Forbid;Replace
type ConcurrencyPolicy string

const (
    // AllowConcurrent allows CronJobs to run concurrently.
    AllowConcurrent ConcurrencyPolicy = "Allow"

    // ForbidConcurrent forbids concurrent runs, skipping next run if previous
    // hasn't finished yet.
    ForbidConcurrent ConcurrencyPolicy = "Forbid"

    // ReplaceConcurrent cancels currently running job and replaces it with a new one.
    ReplaceConcurrent ConcurrencyPolicy = "Replace"
)
```

接下来，让我们设计一下我们的 status，它表示实际看到的状态。它包含了我们希望用户或其他控制器能够轻松获得的任何信息。

我们将保存一个正在运行的 Jobs，以及我们最后一次成功运行 Job 的时间。注意，我们使用 `metav1.Time` 而不是 `time.Time` 来保证序列化的兼容性以及稳定性，如上所述。

```go
// CronJobStatus defines the observed state of CronJob
type CronJobStatus struct {
    // INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
    // Important: Run "make" to regenerate code after modifying this file

    // A list of pointers to currently running jobs.
    // +optional
    Active []corev1.ObjectReference `json:"active,omitempty"`

    // Information when was the last time the job was successfully scheduled.
    // +optional
    LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
}
```

## 了解Controller

CRD是Operator的躯体，那么Controller就是operator的大脑，controller控制资源如何达到期望状态。kubebuilder框架已经帮我们自动生成controller需要的模块，比如reflector，informer，indexer等，这样我们只需关注controller业务逻辑代码。我们只需开发project/controllers/cronjob_controller.go里Reconciler 函数即可。

这里我们修改掉operator里默认日志框架，使用名为 [logr](https://github.com/go-logr/logr) 日志库使用结构化的记录日志，正如我们稍后将看到的，日志记录的工作原理是将键值对附加到静态消息中。我们可以在我们的 Reconcile 方法的顶部预先分配一些配对信息，将他们加入这个 Reconcile 的所有日志中。

```go
// CronJobReconciler reconciles a CronJob object
type CronJobReconciler struct {
    client.Client
    Log    logr.Logger
    Scheme *runtime.Scheme
}

// Reconcile function
func (r *CronJobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
    log := r.Log.WithValues("cronjob", req.NamespacedName)

    // your logic here

    return ctrl.Result{}, nil
}
```

最后，我们将 Reconcile 添加到 manager 中，这样当 manager 启动时它就会被启动。

现在，我们只是注意到这个 Reconcile 是在 `CronJob`s 上运行的。以后，我们也会用这个来标记其他的对象。

```go
// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Complete(r)
}
```

现在我们已经了解了 Reconcile 的基本结构，我们来补充一下 `CronJob`s 的逻辑。

##  实现Controller

CronJob 控制器的基本逻辑如下：

- 根据名称加载定时任务
- 列出所有有效的job，更新其状态
- 根据保留的历史版本数清理版本过旧的Job
- 检查当前CronJob是否被挂起，如果被挂起，则不执行任何操作
- 计算job下一个定时执行时间

注意，我们需要获得RBAC权限——我们需要一些额外权限去 创建和管理job，添加如下一些[字段](https://cloudnative.to/reference/markers/rbac.html)

```go
//+kubebuilder:rbac:groups=batch.sfeng.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.sfeng.io,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.sfeng.io,resources=cronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
```

### Reconciler逻辑

```go
package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/robfig/cron"
	batchv1 "github.com/sfeng/cronjob/api/v1"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"time"
)

type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() }

// clock接口可以获取当前的时间
// 可以帮助我们在测试中模拟计时
type Clock interface {
	Now() time.Time
}

var (
	scheduledTimeAnnotation = "batch.sfeng.io/scheduled-at"
	jobOwnerKey             = ".metadata.controller"
	apiGVStr                = batchv1.GroupVersion.String()
)
```

```go
// CronJobReconciler reconciles a CronJob object
type CronJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Clock
}

//+kubebuilder:rbac:groups=batch.sfeng.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.sfeng.io,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.sfeng.io,resources=cronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile

```

```go
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("cronjob", req.NamespacedName)

	var cronJob batchv1.CronJob
	// 根据名称加载定时任务,如果查不到该对象的话，直接返回，进入下一个调度
	if err := r.Get(ctx, req.NamespacedName, &cronJob); err != nil {
		log.Error(err, "unable to fetch CronJob")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// 列出所有有效 job，更新它们的状态
	// 为确保每个 job 的状态都会被更新到，我们需要列出某个 CronJob 在当前命名空间下的所有 job。 和 Get 方法类似，我们可以使用 List 方法来列出 CronJob 下所有的 job。注意，我们使用变长参数 来映射命名空间和任意多个匹配变量（实际上相当于是建立了一个索引）。
	var childJobs kbatch.JobList
	if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}

	// 找出所有有效的 job
	var activeJobs []*kbatch.Job
	var successfullJobs []*kbatch.Job
	var failedJobs []*kbatch.Job
	var mostRecentTime *time.Time

	getScheduledTimeForJob := func(job *kbatch.Job) (*time.Time, error) {
		timeRaw := job.Annotations[scheduledTimeAnnotation]
		if len(timeRaw) == 0 {
			return nil, nil
		}

		timeParsed, err := time.Parse(time.RFC3339, timeRaw)
		if err != nil {
			return nil, err
		}
		return &timeParsed, nil
	}

	isJobFinished := func(job *kbatch.Job) (bool, kbatch.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}
		return false, ""
	}

	for i, job := range childJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "":
			activeJobs = append(activeJobs, &childJobs.Items[i])
		case kbatch.JobFailed:
			failedJobs = append(failedJobs, &childJobs.Items[i])
		case kbatch.JobComplete:
			successfullJobs = append(successfullJobs, &childJobs.Items[i])
		}
		//将启动时间存放在注释中，当job生效时可以从中读取
		scheduledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			log.Error(err, "unable to parse schedule time for child job", "job", &job)
			continue
		}
		if scheduledTimeForJob != nil {
			if mostRecentTime == nil {
				mostRecentTime = scheduledTimeForJob
			} else if mostRecentTime.Before(*scheduledTimeForJob) {
				mostRecentTime = scheduledTimeForJob
			}
		}
	}
	if mostRecentTime != nil {
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		cronJob.Status.LastScheduleTime = nil
	}
	cronJob.Status.Active = nil
	for _, activeJob := range activeJobs {
		jobRef, err := reference.GetReference(r.Scheme, activeJob)
		if err != nil {
			log.Error(err, "unable to make reference to active job", "job", activeJob)
			continue
		}
		cronJob.Status.Active = append(cronJob.Status.Active, *jobRef)
	}
	log.V(1).Info("job count", "active jobs", len(activeJobs), "successful jobs", len(successfullJobs), "failed jobs", len(failedJobs))

	if err := r.Status().Update(ctx, &cronJob); err != nil {
		log.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, err
	}

	// 我们先清理掉一些版本太旧的 job，这样可以不用保留太多无用的 job
	// 注意: 删除操作采用的“尽力而为”策略
	// 如果个别 job 删除失败了，不会将其重新排队，直接结束删除操作
	if cronJob.Spec.FailedJobsHistoryLimit != nil {
		sort.Slice(failedJobs, func(i, j int) bool {
			if failedJobs[i].Status.StartTime == nil {
				return failedJobs[j].Status.StartTime != nil
			}
			return failedJobs[i].Status.StartTime.Before(failedJobs[j].Status.StartTime)
		})
		for i, job := range failedJobs {
			if int32(i) >= int32(len(failedJobs))-*cronJob.Spec.FailedJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete old failed job", "job", job)
			} else {
				log.V(0).Info("deleted old failed job", "job", job)
			}
		}
	}

	if cronJob.Spec.SuccessfulJobsHistoryLimit != nil {
		sort.Slice(successfullJobs, func(i, j int) bool {
			if successfullJobs[i].Status.StartTime == nil {
				return successfullJobs[j].Status.StartTime != nil
			}
			return successfullJobs[i].Status.StartTime.Before(successfullJobs[j].Status.StartTime)
		})
		for i, job := range successfullJobs {
			if int32(i) >= int32(len(successfullJobs))-*cronJob.Spec.SuccessfulJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
				log.Error(err, "unable to delete old successful job", "job", job)
			} else {
				log.V(0).Info("deleted old successful job", "job", job)
			}
		}
	}

	// 如果当前 cronjob 被挂起，不会再运行其下的任何 job，我们将其停止。这对于某些 job 出现异常 的排查非常有用。我们无需删除 cronjob 来中止其后续其他 job 的运行。
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		log.V(1).Info("cronjob suspended, skipping")
		return ctrl.Result{}, nil
	}

	getNextSchedule := func(cronJob *batchv1.CronJob, now time.Time) (lastMissed time.Time, next time.Time, err error) {
		sched, err := cron.ParseStandard(cronJob.Spec.Schedule)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("Unparseable schedule %q: %v", cronJob.Spec.Schedule, err)
		}

		// 出于优化的目的，我们可以使用点技巧。从上一次观察到的执行时间开始执行，
		// 这个执行时间可以被在这里被读取。但是意义不大，因为我们刚更新了这个值。

		var earliestTime time.Time
		if cronJob.Status.LastScheduleTime != nil {
			earliestTime = cronJob.Status.LastScheduleTime.Time
		} else {
			earliestTime = cronJob.ObjectMeta.CreationTimestamp.Time
		}
		if cronJob.Spec.StartingDeadlineSeconds != nil {
			// 如果开始执行时间超过了截止时间，不再执行
			schedulingDeadline := now.Add(-time.Second * time.Duration(*cronJob.Spec.StartingDeadlineSeconds))

			if schedulingDeadline.After(earliestTime) {
				earliestTime = schedulingDeadline
			}
		}
		if earliestTime.After(now) {
			return time.Time{}, sched.Next(now), nil
		}

		starts := 0
		for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
			lastMissed = t
			// 一个 CronJob 可能会遗漏多次执行。举个例子，周五 5:00pm 技术人员下班后，
			// 控制器在 5:01pm 发生了异常。然后直到周二早上才有技术人员发现问题并
			// 重启控制器。那么所有的以1小时为周期执行的定时任务，在没有技术人员
			// 进一步的干预下，都会有 80 多个 job 在恢复正常后一并启动（如果 job 允许
			// 多并发和延迟启动）

			// 如果 CronJob 的某些地方出现异常，控制器或 apiservers (用于设置任务创建时间)
			// 的时钟不正确, 那么就有可能出现错过很多次执行时间的情形（跨度可达数十年）
			// 这将会占满控制器的CPU和内存资源。这种情况下，我们不需要列出错过的全部
			// 执行时间。

			starts++
			if starts > 100 {
				// 获取不到最近一次执行时间，直接返回空切片
				return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100). Set or decrease .spec.startingDeadlineSeconds or check clock skew.")
			}
		}
		return lastMissed, sched.Next(now), nil
	}
	// 计算出定时任务下一次执行时间（或是遗漏的执行时间）
	missedRun, nextRun, err := getNextSchedule(&cronJob, r.Now())
	if err != nil {
		log.Error(err, "unable to figure out CronJob schedule")
		// 重新排队直到有更新修复这次定时任务调度，不必返回错误
		return ctrl.Result{}, nil
	}
	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())} // 保存以便别处复用
	log = log.WithValues("now", r.Now(), "next run", nextRun)

	if missedRun.IsZero() {
		log.V(1).Info("no upcoming scheduled times, sleeping until next")
		return scheduledResult, nil
	}

	// 确保错过的执行没有超过截止时间
	log = log.WithValues("current run", missedRun)
	tooLate := false
	if cronJob.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*cronJob.Spec.StartingDeadlineSeconds) * time.Second).Before(r.Now())
	}
	if tooLate {
		log.V(1).Info("missed starting deadline for last run, sleeping till next")
		// TODO(directxman12): events
		return scheduledResult, nil
	}

	// 确定要 job 的执行策略 —— 并发策略可能禁止多个job同时运行
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ForbidConcurrent && len(activeJobs) > 0 {
		log.V(1).Info("concurrency policy blocks concurrent runs, skipping", "num active", len(activeJobs))
		return scheduledResult, nil
	}

	// 直接覆盖现有 job
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ReplaceConcurrent {
		for _, activeJob := range activeJobs {
			// we don't care if the job was already deleted
			if err := r.Delete(ctx, activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete active job", "job", activeJob)
				return ctrl.Result{}, err
			}
		}
	}

	constructJobForCronJob := func(cronJob *batchv1.CronJob, scheduledTime time.Time) (*kbatch.Job, error) {
		// job 名称带上执行时间以确保唯一性，避免排定执行时间的 job 创建两次
		name := fmt.Sprintf("%s-%d", cronJob.Name, scheduledTime.Unix())

		job := &kbatch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        name,
				Namespace:   cronJob.Namespace,
			},
			Spec: *cronJob.Spec.JobTemplate.Spec.DeepCopy(),
		}
		for k, v := range cronJob.Spec.JobTemplate.Annotations {
			job.Annotations[k] = v
		}
		job.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
		for k, v := range cronJob.Spec.JobTemplate.Labels {
			job.Labels[k] = v
		}
		if err := ctrl.SetControllerReference(cronJob, job, r.Scheme); err != nil {
			return nil, err
		}

		return job, nil
	}

	// 构建 job
	job, err := constructJobForCronJob(&cronJob, missedRun)
	if err != nil {
		log.Error(err, "unable to construct job from template")
		// job 的 spec 没有变更，无需重新排队
		return scheduledResult, nil
	}

	// ...在集群中创建 job
	if err := r.Create(ctx, job); err != nil {
		log.Error(err, "unable to create Job for CronJob", "job", job)
		return ctrl.Result{}, err
	}

	log.V(1).Info("created Job for CronJob run", "job", job)
	// 当有 job 进入运行状态后，重新排队，同时更新状态
	return scheduledResult, nil

}
```

```go
// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 此处不是测试，我们需要创建一个真实的时钟
	if r.Clock == nil {
		r.Clock = realClock{}
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kbatch.Job{}, jobOwnerKey, func(rawObj client.Object) []string {
		//获取 job 对象，提取 owner...
		job := rawObj.(*kbatch.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...确保 owner 是个 CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "CronJob" {
			return nil
		}

		// ...是 CronJob，返回
		return []string{owner.Name}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Owns(&kbatch.Job{}).
		Complete(r)
}
```

## 部署

以下所有操作命令都在项目根目录执行，即./cronjob

### 制作docker image

build images之前，先修改Dockerfile，增加国内goproxy，提高gomodule下载速度。

```docker
# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# env
ENV GOPROXY https://goproxy.cn,direct

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
```

制作镜像

```bash
# 制作docker image
$ make docker-build
# 如果无需推送镜像至镜像仓库，这步可省略
$ make docker-push
```

### 发布crd

```
$ make install 
```

###  部署controller

```bash
# 这里会出现拉取gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0 失败的情况，所以需要提前拉取国内镜像并更换tag
$ docker pull kubesphere/kube-rbac-proxy:v0.8.0
$ docker tag kubesphere/kube-rbac-proxy:v0.8.0 gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0
$ make deploy
$ kubectl get pods -n cronjob-system
NAME                                          READY   STATUS    RESTARTS   AGE
cronjob-controller-manager-6ff888b854-jqw4z   2/2     Running   0          46s
```

###  部署cr

这里创建一个一分钟执行一个任务的job

```yaml
# 一下内容放入config/samples/batch_v1_cronjob.yaml
apiVersion: batch.sfeng.io/v1
kind: CronJob
metadata:
  name: cronjob-sample
spec:
  schedule: "*/1 * * * *"
  startingDeadlineSeconds: 60
  concurrencyPolicy: Allow # explicitly specify, but Allow is also default.
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox
            args:
            - /bin/sh
            - -c
            - date; echo Hello from the Kubernetes cluster
          restartPolicy: OnFailure
```

```
kubectl create -f config/samples/batch_v1_cronjob.yaml 
```

###  测试

```bash
# 可以发现一分钟执行一次任务，任务执行完成pod即停止。
$ kubectl get cronjobs.batch.sfeng.io -n default
NAME             AGE
cronjob-sample   2m38s
$ kubectl get job -n default
NAME                        COMPLETIONS   DURATION   AGE
cronjob-sample-1638168120   1/1           16s        2m15s
cronjob-sample-1638168180   1/1           6s         75s
cronjob-sample-1638168240   1/1           6s         15s
$ kubectl get pods -n default
NAME                              READY   STATUS      RESTARTS   AGE
cronjob-sample-1638168120-66djz   0/1     Completed   0          2m22s
cronjob-sample-1638168180-czr4j   0/1     Completed   0          82s
cronjob-sample-1638168240-tqzvt   0/1     Completed   0          22s
$ kubectl logs -f pods/cronjob-sample-1638168120-66djz -n default
Mon Nov 29 06:42:15 UTC 2021
Hello from the Kubernetes cluster
```

## 总结

上面只是演示可kubebuilder的基本功能，当然大部分operator依靠上述操作基本就能完成，kubebuilder还可以创建webhook，多版本API管理等功能。所以我们在开发kuberntes operator时，kubebuilder是一个很好地选择，高效，简洁。

## 引用

[kubebuilder中文文档](https://cloudnative.to/kubebuilder/)

[kubernetes官方文档](https://kubernetes.io/zh/docs/concepts/extend-kubernetes/operator/)

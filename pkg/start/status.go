package start

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func waitUntilPodsRunning(c kubernetes.Interface, pods map[string][]string, timeout time.Duration) error {
	sc, err := newStatusController(c, pods)
	if err != nil {
		return err
	}
	sc.Run()

	if err := wait.Poll(5*time.Second, timeout, sc.AllRunningAndReady); err != nil {
		return fmt.Errorf("error while checking pod status: %v", err)
	}

	UserOutput("All self-hosted control plane components successfully started\n")
	return nil
}

type statusController struct {
	client           kubernetes.Interface
	podStore         cache.Store
	watchPodPrefixes map[string][]string
	lastPodPhases    map[string]*podStatus
}

func newStatusController(client kubernetes.Interface, pods map[string][]string) (*statusController, error) {
	return &statusController{client: client, watchPodPrefixes: pods}, nil
}

func (s *statusController) Run() {
	// TODO(yifan): Be more explicit about the labels so that we don't just
	// reply on the prefix of the pod name when looking for the pods we are interested.
	// E.g. For a scheduler pod, we will look for pods that has label `tier=control-plane`
	// and `component=kube-scheduler`.
	options := metav1.ListOptions{}
	podStore, podController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (runtime.Object, error) {
				return s.client.Core().Pods("").List(options)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return s.client.Core().Pods("").Watch(options)
			},
		},
		&v1.Pod{},
		30*time.Minute,
		cache.ResourceEventHandlerFuncs{},
	)
	s.podStore = podStore
	go podController.Run(wait.NeverStop)
}

func (s *statusController) AllRunningAndReady() (bool, error) {
	ps, err := s.podStatus()
	if err != nil {
		glog.Infof("Error retriving pod statuses: %v", err)
		return false, nil
	}

	if s.lastPodPhases == nil {
		s.lastPodPhases = ps
	}

	// use lastPodPhases to print only pods whose phase has changed
	changed := !reflect.DeepEqual(ps, s.lastPodPhases)
	s.lastPodPhases = ps

	runningAndReady := true
	for p, s := range ps {
		if changed {
			var status string
			switch {
			case s == nil:
				status = "DoesNotExist"
			case s.Phase == v1.PodRunning && s.IsReady:
				status = "Ready"
			case s.Phase == v1.PodRunning && !s.IsReady:
				status = "RunningNotReady"
			default:
				status = string(s.Phase)
			}

			UserOutput("\tPod Status:%24s\t%s\n", p, status)
		}
		if s == nil || s.Phase != v1.PodRunning || !s.IsReady {
			runningAndReady = false
		}
	}
	return runningAndReady, nil
}

// podStatus describes a pod's phase and readiness.
type podStatus struct {
	Phase   v1.PodPhase
	IsReady bool
}

// podStatus retrieves the pod status by reading the PodPhase and whether it is ready.
// A non existing pod is represented with nil.
func (s *statusController) podStatus() (map[string]*podStatus, error) {
	status := make(map[string]*podStatus)

	podNames := s.podStore.ListKeys()
	for desc, prefixes := range s.watchPodPrefixes {
		// Prefixes names are suffixed with random data. Match on prefix
		var podName string
	found:
		for _, pn := range podNames {
			for _, prefix := range prefixes {
				if strings.HasPrefix(pn, prefix) {
					podName = pn
					break found
				}
			}
		}
		exists := false
		var p interface{}
		if len(podName) > 0 {
			var err error
			if p, exists, err = s.podStore.GetByKey(podName); err != nil {
				return nil, err
			}
		}
		if !exists {
			status[desc] = nil
			continue
		}
		if p, ok := p.(*v1.Pod); ok {
			status[desc] = &podStatus{
				Phase: p.Status.Phase,
			}
			for _, c := range p.Status.Conditions {
				if c.Type == v1.PodReady {
					status[desc].IsReady = c.Status == v1.ConditionTrue
				}
			}
		}
	}
	return status, nil
}

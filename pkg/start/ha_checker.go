package start

import (
	"context"
	"fmt"
	"sync"
	"time"

	operatorversionedclient "github.com/openshift/client-go/operator/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// waitForSelfHostedControlPlaneAvailabilityBeforeTearDown will wait until the
// following conditions are true:
// a) at least two master nodes have API available
// b) at least one master node has scheduler installed
// c) at least one master node has kcm installed
func waitForSelfHostedControlPlaneAvailabilityBeforeTearDown(loopbackOperatorClient operatorversionedclient.Interface, timeout time.Duration) error {
	return waitFor([]*poller{
		newAPIAvailabilityPoller(loopbackOperatorClient, timeout),
		newSchedulerAvailabilityPoller(loopbackOperatorClient, timeout),
		newKCMAvailabilityPoller(loopbackOperatorClient, timeout),
	})
}

func waitFor(pollers []*poller) error {
	wg := sync.WaitGroup{}
	wg.Add(len(pollers))

	errCh := make(chan error, len(pollers))
	for i := range pollers {
		p := pollers[i]
		go func(p *poller) {
			defer wg.Done()
			if err := p.poll(); err != nil {
				errCh <- err
			}
		}(p)
	}

	wg.Wait()
	close(errCh)

	errs := make([]error, 0)
	for err := range errCh {
		errs = append(errs, err)
	}
	return utilerrors.NewAggregate(errs)
}

type poller struct {
	// description of the condition
	what    string
	timeout time.Duration
	// if the function returns true, it indicates the given condition
	// has been satisfied; otherwise a false indicates the condition
	// hasn't been satisfied yet, and polling should continue
	condition func(context.Context) (reason string, satisfied bool)
}

func (p poller) poll() error {
	UserOutput("Waiting up to %s for condition: %s\n", p.timeout, p.what)
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	lastMsg := ""
	err := wait.PollUntil(2*time.Second, func() (bool, error) {
		reason, satisfied := p.condition(ctx)
		if satisfied {
			UserOutput("condition %q has been satisfied, reason: %s\n", p.what, reason)
			return true, nil
		}

		if len(reason) > 0 {
			msg := fmt.Sprintf("polling will continue, last status: %s\n", reason)
			if msg != lastMsg {
				UserOutput(msg)
				lastMsg = msg
			}
		}
		return false, nil
	}, ctx.Done())
	if err != nil {
		return fmt.Errorf("time out waiting for condition: %q, err: %w", p.what, err)
	}

	return nil
}

func newAPIAvailabilityPoller(loopbackOperatorClient operatorversionedclient.Interface, timeout time.Duration) *poller {
	return &poller{
		timeout: timeout,
		what:    "API should be available on at least two master nodes",
		condition: func(ctx context.Context) (string, bool) {
			client := loopbackOperatorClient.OperatorV1().KubeAPIServers()
			config, err := client.Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return fmt.Sprintf("error getting kubeapiservers/cluster - %v", err), false
			}

			statuses := config.Status.NodeStatuses
			if len(statuses) == 0 {
				return fmt.Sprintf("NodeStatuses for kubeapiservers/cluster is empty"), false
			}

			available := 0
			msg := ""
			for _, status := range statuses {
				msg = fmt.Sprintf("%s [%s at Current: %d, Target: %d]", msg, status.NodeName, status.CurrentRevision, status.TargetRevision)
				if status.CurrentRevision >= 1 {
					available++
				}
			}

			return fmt.Sprintf("kubeapiservers/cluster NodeStatuses: %s", msg), available >= 2
		},
	}
}

func newSchedulerAvailabilityPoller(loopbackOperatorClient operatorversionedclient.Interface, timeout time.Duration) *poller {
	return &poller{
		timeout: timeout,
		what:    "scheduler should be available on at least two master nodes",
		condition: func(ctx context.Context) (string, bool) {
			client := loopbackOperatorClient.OperatorV1().KubeSchedulers()
			config, err := client.Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return fmt.Sprintf("error getting kubeschedulers/cluster - %v", err), false
			}

			statuses := config.Status.NodeStatuses
			if len(statuses) == 0 {
				return fmt.Sprintf("NodeStatuses for kubeschedulers/cluster is empty"), false
			}

			available := 0
			msg := ""
			for _, status := range statuses {
				msg = fmt.Sprintf("%s [%s at Current: %d, : %d]", msg, status.NodeName, status.CurrentRevision, status.TargetRevision)
				if status.CurrentRevision >= 1 {
					available++
				}
			}

			return fmt.Sprintf("kubeschedulers/cluster NodeStatuses: %s", msg), available >= 2
		},
	}
}

func newKCMAvailabilityPoller(loopbackOperatorClient operatorversionedclient.Interface, timeout time.Duration) *poller {
	return &poller{
		timeout: timeout,
		what:    "kcm should be available on at least two master nodes",
		condition: func(ctx context.Context) (string, bool) {
			client := loopbackOperatorClient.OperatorV1().KubeControllerManagers()
			config, err := client.Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return fmt.Sprintf("error getting kubecontrollermanagers/cluster - %v", err), false
			}

			statuses := config.Status.NodeStatuses
			if len(statuses) == 0 {
				return fmt.Sprintf("NodeStatuses for kubecontrollermanagers/cluster is empty"), false
			}

			available := 0
			msg := ""
			for _, status := range statuses {
				msg = fmt.Sprintf("%s [%s at Current: %d, : %d]", msg, status.NodeName, status.CurrentRevision, status.TargetRevision)
				if status.CurrentRevision >= 1 {
					available++
				}
			}
			return fmt.Sprintf("kubecontrollermanagers/cluster NodeStatuses: %s", msg), available >= 2
		},
	}
}

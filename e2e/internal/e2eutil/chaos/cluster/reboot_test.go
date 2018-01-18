package cluster

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// global client to be used by all tests.
	client kubernetes.Interface
)

func TestMain(m *testing.M) {
	var kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Set("logtostderr", "true")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestMaxDisruption(t *testing.T) {
	if err := ready(client); err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	cluster, err := New(client,
		WithMaxDisruption(1),
	)
	if err != nil {
		t.Fatal(err)
	}

	doneCh := make(chan struct{})
	go func() {
		defer func() { doneCh <- struct{}{} }()
		if err := cluster.RebootMasters(2 * time.Minute); err != nil {
			t.Fatal(err)
		}
	}()

	hosts := hostsFromNodes(cluster.Masters)
	sshClient := utils.MustNewSSHClient(&utils.SSHConfig{Timeout: 10 * time.Second})
	if err := wait.PollInfinite(10*time.Second, func() (bool, error) {
		select {
		case <-doneCh:
			return true, nil
		default:
		}

		rebooting := 0
		for _, host := range hosts {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _, err := sshClient.ExecWithCtx(ctx, host, "sudo systemctl is-active kubelet")
			if err != nil {
				rebooting++
			}
			cancel()
		}
		if rebooting > 1 {
			return false, fmt.Errorf("more than 1 node rebooting")
		}

		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRebootAll(t *testing.T) {
	if err := ready(client); err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	cluster, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	doneCh := make(chan struct{})
	go func() {
		defer func() { doneCh <- struct{}{} }()
		if err := cluster.RebootAll(2 * time.Minute); err != nil {
			t.Fatal(err)
		}
	}()

	hosts := hostsFromNodes(cluster.Masters)
	hosts = append(hosts, hostsFromNodes(cluster.Workers)...)
	checkAllRebooting(t, hosts)
	<-doneCh
}

func TestRebootMasters(t *testing.T) {
	if err := ready(client); err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	cluster, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	doneCh := make(chan struct{})
	go func() {
		defer func() { doneCh <- struct{}{} }()
		if err := cluster.RebootMasters(2 * time.Minute); err != nil {
			t.Fatal(err)
		}
	}()

	hosts := hostsFromNodes(cluster.Masters)
	checkAllRebooting(t, hosts)
	<-doneCh
}

func TestRebootWorkers(t *testing.T) {
	if err := ready(client); err != nil {
		t.Fatalf("cluster not ready: %v", err)
	}

	cluster, err := New(client)
	if err != nil {
		t.Fatal(err)
	}
	doneCh := make(chan struct{})
	go func() {
		defer func() { doneCh <- struct{}{} }()
		if err := cluster.RebootWorkers(2 * time.Minute); err != nil {
			t.Fatal(err)
		}
	}()

	hosts := hostsFromNodes(cluster.Workers)
	checkAllRebooting(t, hosts)
	<-doneCh
}

func checkAllRebooting(t *testing.T, hosts []string) {
	sshClient := utils.MustNewSSHClient(&utils.SSHConfig{Timeout: 10 * time.Second})
	if err := wait.PollImmediate(10*time.Second, 3*time.Minute, func() (bool, error) {
		rebooting := 0
		for _, host := range hosts {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _, err := sshClient.ExecWithCtx(ctx, host, "sudo systemctl is-active kubelet")
			if err != nil {
				rebooting++
			}
			cancel()
		}
		if rebooting != len(hosts) {
			return false, nil
		}

		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// ready blocks until the cluster is considered available. The current
// implementation checks that 1 schedulable node is ready.
func ready(c kubernetes.Interface) error {
	f := func() error {
		list, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(list.Items) < 1 {
			return fmt.Errorf("cluster is not ready, waiting for 1 or more nodes: %v", len(list.Items))
		}

		// check for 1 or more ready nodes by ignoring nodes marked
		// unschedulable or containing taints
		var oneReady bool
		for _, node := range list.Items {
			if node.Spec.Unschedulable {
				continue
			}
			if len(node.Spec.Taints) != 0 {
				continue
			}
			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady {
					if condition.Status == v1.ConditionTrue {
						oneReady = true
					}
					break
				}
			}
		}
		if !oneReady {
			return fmt.Errorf("waiting for atleast one node to be ready")
		}
		return nil
	}

	if err := utils.Retry(50, 10*time.Second, f); err != nil {
		return err
	}
	return nil
}

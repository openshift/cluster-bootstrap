package e2e

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

// Reboot all nodes in cluster all at once. Wait for nodes to return. Run nginx
// workload.
func TestReboot(t *testing.T) {
	nodeList, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	for _, node := range nodeList.Items {
		var host string
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeExternalIP {
				host = addr.Address
				break
			}
		}
		if host == "" {
			t.Skip("Could not get external node IP, kubelet must use cloud-provider flags")
		}

		// reboot
		_, _, err := sshClient.SSH(host, "sudo reboot")
		if _, ok := err.(*ssh.ExitMissingError); ok {
			err = nil
		}

		if err != nil {
			t.Fatalf("rebooting node: %v", err)
		}
	}

	// make sure nodes have chance to go down
	time.Sleep(15 * time.Second)

	if err := nodesReady(client, len(nodeList.Items), t); err != nil {
		t.Fatalf("Some or all nodes did not recover from reboot: %v", err)
	}

}

// block until n nodes are ready
func nodesReady(c kubernetes.Interface, expectedNodes int, t *testing.T) error {
	f := func() error {
		list, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		if len(list.Items) != expectedNodes {
			return fmt.Errorf("cluster is not ready, expected %v nodes got %v", expectedNodes, len(list.Items))
		}

		for _, node := range list.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady {
					if condition.Status != v1.ConditionTrue {
						return fmt.Errorf("One or more nodes not in the ready state: %v", node.Status.Phase)
					}
					break
				}
			}
		}

		return nil
	}

	if err := retry(40, 10*time.Second, f); err != nil {
		return err
	}
	return nil
}

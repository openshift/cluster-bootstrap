package tests

import (
	"testing"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/log-collector/pkg/fluentd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

func TestFluentdMasterAddress(t *testing.T) {
	host, err := fluentd.GetNodeAddressWithMaster(client, namespace)
	if err != nil {
		t.Fatalf("error getting master's node address: %v", err)
	}

	nl, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "log-collector.github.com/fluentd-master"})
	if err != nil || len(nl.Items) == 0 {
		t.Fatalf("error getting master nodes: %v", err)
	}
	for _, node := range nl.Items {
		var h string
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeExternalIP {
				h = addr.Address
				break
			}
		}
		if h == "" {
			for _, addr := range node.Status.Addresses {
				if addr.Type == "LegacyHostIP" {
					h = addr.Address
					break
				}
			}
		}
		if h == "" {
			t.Fatal("could not get external node IP of master node")
		}
		if h == host {
			t.Logf("Found matching master node: %s", host)
			return
		}
	}

	t.Fatalf("host: %s is not one of the masters.", host)
}

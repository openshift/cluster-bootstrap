package e2e

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	kubeEtcdTPRURI = "/apis/etcd.coreos.com/v1beta1/namespaces/kube-system/clusters/kube-etcd"
	pollTimeout    = 5 * time.Minute
	pollInterval   = 5 * time.Second
)

func TestEtcdScale(t *testing.T) {
	if err := etcdScalePreCheck(3); err != nil {
		t.Skip(err)
	}
	t.Run("ResizeSelfHostedEtcdTo3", func(t *testing.T) { resizeSelfHostedEtcd(t, 3) })
	t.Run("CheckEtcdPodDistribution", func(t *testing.T) { checkEtcdPodDistribution(t, 3) })
	if *enableExperimental {
		// Experimental: currently does not work reliably.
		// See: https://github.com/kubernetes-incubator/bootkube/issues/656.
		t.Run("ResizeSelfHostedEtcdTo1", func(t *testing.T) { resizeSelfHostedEtcd(t, 1) })
	} else {
		t.Log("ResizeSelfHostedEtcdTo1: skipped because enable-experimental is false.")
	}
}

// etcdScalePreCheck determines if the etcd scale tests should run. It returns an error if the tests
// should be skipped.
//
// Criteria for running:
// - etcd-operator is running.
// - enough master nodes are available to accommodate new etcd pods.
func etcdScalePreCheck(requiredMasters int) error {
	// Check for etcd-operator by getting pod
	l, err := client.CoreV1().Pods("kube-system").List(metav1.ListOptions{LabelSelector: "k8s-app=etcd-operator"})
	if err != nil || len(l.Items) == 0 {
		return fmt.Errorf("test requires a cluster with self-hosted etcd: %v", err)
	}

	// Check if we are expecting enough master nodes.
	if expectedMasters < requiredMasters {
		return fmt.Errorf("test requires %d masters, test was run expecting %d", requiredMasters, expectedMasters)
	}

	// Wait until enough master nodes are ready.
	return wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		list, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
		if err != nil {
			log.Printf("Error listing nodes: %v\n", err)
			return false, nil
		}
		if foundMasters := len(list.Items); foundMasters < requiredMasters {
			log.Printf("Need %d master nodes available for etcd scale, got: %d\n", requiredMasters, foundMasters)
			return false, nil
		}
		readyMasters := 0
		for _, node := range list.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
					readyMasters++
				}
			}
		}
		if readyMasters < requiredMasters {
			log.Printf("Need %d master nodes ready for etcd scale test, got: %d\n", requiredMasters, readyMasters)
			return false, nil
		}
		return true, nil
	})
}

// resizes self-hosted etcd and checks that the desired number of pods are in a running state
func resizeSelfHostedEtcd(t *testing.T, size int) {
	httpRestClient := client.ExtensionsV1beta1().RESTClient()
	var tpr unstructured.Unstructured

	// Resize cluster by updating TPR.
	if err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		// get tpr
		b, err := httpRestClient.Get().RequestURI(kubeEtcdTPRURI).DoRaw()
		if err != nil {
			log.Printf("Failed to get TPR: %v\n", err)
			return false, nil
		}

		if err := json.Unmarshal(b, &tpr); err != nil {
			log.Printf("Failed to unmarshal TPR: %v\n", err)
			return false, nil
		}

		// change size
		spec, ok := tpr.Object["spec"].(map[string]interface{})
		if !ok {
			log.Println("Could not get 'spec' from TPR")
			return false, nil
		}
		spec["size"] = size

		// update tpr
		data, err := json.Marshal(&tpr)
		if err != nil {
			log.Printf("Could not marshal TPR: %v\n", err)
			return false, nil
		}

		result := httpRestClient.Put().RequestURI(kubeEtcdTPRURI).Body(data).Do()
		if err := result.Error(); err != nil {
			log.Printf("Error updating TPR: %v\n", err)
			return false, nil
		}
		var statusCode int
		result.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			log.Printf("Unexpected status code %d, expecting %d\n", statusCode, http.StatusOK)
			return false, nil
		}

		return true, nil
	}); err != nil {
		t.Fatalf("Failed to scale cluster: %v", err)
	}

	// Check that all pods are running by checking TPR.
	if err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		// get tpr
		b, err := httpRestClient.Get().RequestURI(kubeEtcdTPRURI).DoRaw()
		if err != nil {
			log.Printf("Failed to get TPR: %v\n", err)
			return false, nil
		}

		if err := json.Unmarshal(b, &tpr); err != nil {
			log.Printf("Failed to unmarshal TPR: %v\n", err)
			return false, nil
		}

		// check status of members
		status, ok := tpr.Object["status"].(map[string]interface{})
		if !ok {
			log.Println("Could not asset 'status' type from TPR")
			return false, nil
		}
		members, ok := status["members"].(map[string]interface{})
		if !ok {
			log.Println("Could not assert 'members' type from TPR")
			return false, nil
		}
		readyList, ok := members["ready"].([]interface{})
		if !ok {
			log.Println("Could not assert 'ready' type from TPR")
			return false, nil
		}

		// check that we have enough nodes considered ready by operator
		if len(readyList) != size {
			log.Printf("Expected %d etcd pods got %d: %v\n", size, len(readyList), readyList)
			return false, nil
		}

		return true, nil
	}); err != nil {
		t.Errorf("Waited 300 seconds for etcd to scale: %v", err)
	}
}

// Checks that self-hosted etcd pods are scheduled on different master nodes
// when possible. Look at the number of unique nodes etcd pods are scheduled
// on and compare to the lesser value between total number of master nodes and
// total number of etcd pods.
func checkEtcdPodDistribution(t *testing.T, etcdClusterSize int) {
	// get pods
	pods, err := client.CoreV1().Pods("kube-system").List(metav1.ListOptions{LabelSelector: "etcd_cluster=kube-etcd"})
	if err != nil {
		t.Fatalf("Error getting etcd pods: %v", err)
	}
	if len(pods.Items) != etcdClusterSize {
		t.Fatalf("Wanted %d etcd pods, got: %d", etcdClusterSize, len(pods.Items))
	}
	// get master nodes
	mnodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
	if err != nil {
		t.Fatalf("Error listing nodes: %v", err)
	}

	// set of nodes pods are running on identified by HostIP
	nodeSet := map[string]struct{}{}
	for _, pod := range pods.Items {
		nodeSet[pod.Status.HostIP] = struct{}{}
	}

	var expectedUniqueNodes int
	if len(mnodes.Items) > etcdClusterSize {
		expectedUniqueNodes = etcdClusterSize
	} else {
		expectedUniqueNodes = len(mnodes.Items)
	}

	if len(nodeSet) != expectedUniqueNodes {
		t.Errorf("Self-hosted etcd pods not properly distributed")
	}

	// check that each node in nodeSet is a master node
	masterSet := map[string]struct{}{}
	for _, node := range mnodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				masterSet[addr.Address] = struct{}{}
				break
			}
		}
	}

	for k := range nodeSet {
		if _, ok := masterSet[k]; !ok {
			t.Errorf("Detected self-hosted etcd pod running on non-master node %v %v", masterSet, nodeSet)
		}
	}
}

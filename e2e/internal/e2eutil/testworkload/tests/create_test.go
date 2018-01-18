package tests

import (
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/testworkload"
)

func TestCreate(t *testing.T) {
	n, err := testworkload.NewNginx(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Delete()

	t.Run("CheckDeployExists", checkDeployExists(n))
	t.Run("CheckRunningPodsCount", checkRunningPodsCount(n))
	t.Run("CheckSerivceExists", checkSerivceExists(n))
}

func checkDeployExists(n *testworkload.Nginx) func(*testing.T) {
	return func(t *testing.T) {
		_, err := client.ExtensionsV1beta1().Deployments(namespace).Get(n.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found deployment %s: %v", n.Name, err)
			}
			t.Fatalf("error finding deployment %s: %v", n.Name, err)
		}
	}
}

func checkRunningPodsCount(n *testworkload.Nginx) func(*testing.T) {
	return func(t *testing.T) {
		d, err := client.ExtensionsV1beta1().Deployments(namespace).Get(n.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found deployment %s: %v", n.Name, err)
			}
			t.Fatalf("error finding deployment %s: %v", n.Name, err)
		}

		pl, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", n.Name)})
		if err != nil {
			t.Fatalf("error getting pods for deployment %s: %v", n.Name, err)
		}

		if int(d.Status.UpdatedReplicas) != len(pl.Items) {
			t.Fatalf("needed pods: %d found: %d", d.Status.UpdatedReplicas, len(pl.Items))
		}
	}
}

func checkSerivceExists(n *testworkload.Nginx) func(*testing.T) {
	return func(t *testing.T) {
		_, err := client.CoreV1().Services(namespace).Get(n.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Fatalf("not found service %s: %v", n.Name, err)
			}
			t.Fatalf("error finding service %s: %v", n.Name, err)
		}
	}
}

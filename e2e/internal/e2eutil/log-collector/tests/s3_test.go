package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/utils"

	"os"
)

func TestS3Output(t *testing.T) {
	empty := func() error {
		return fmt.Errorf("format string")
	}
	utils.Retry(10, time.Second*10, empty)

	t.Run("Pod", testS3Pod)
	t.Run("Service", testS3Service)
}

func testS3Pod(t *testing.T) {
	if err := cr.SetOutputToS3(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_REGION"), "abhinav-log-collector", "a1"); err != nil {
		t.Fatalf("error connecting to s3 %v", err)
	}

	results, err := cr.CollectPodLogs("kube-apiserver")
	if err != nil {
		t.Fatalf("error in collecting %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("alteast one container exists but not found")
	}
	t.Log(results)
}

func testS3Service(t *testing.T) {
	if err := cr.SetOutputToS3(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_REGION"), "abhinav-log-collector", "a1"); err != nil {
		t.Fatalf("error connecting to s3 %v", err)
	}

	results, err := cr.CollectServiceLogs("kubelet")
	if err != nil {
		t.Fatalf("error in collecting %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("alteast one service exists but not found")
	}
	t.Log(results)
}

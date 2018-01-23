package e2e

import (
	"log"
	"os"
	"path/filepath"

	"fmt"

	collector "github.com/kubernetes-incubator/bootkube/e2e/internal/e2eutil/log-collector"
)

var cr *collector.Collector

func startLogging(keypath, dstDir string) error {
	cr = collector.New(&collector.Config{
		K8sClient:     client,
		Namespace:     namespace,
		RemoteKeyFile: keypath,
	})
	if err := cr.Start(); err != nil {
		return fmt.Errorf("error starting log-collector: %v", err)
	}

	dirAbsPath, _ := filepath.Abs(dstDir)
	os.Mkdir(dirAbsPath, 0777)
	if err := cr.SetOutputToLocal(dirAbsPath); err != nil {
		return fmt.Errorf("error starting log-collector: %v", err)
	}
	return nil
}

func collectLogs() error {
	pods, err := cr.CollectPodLogs("*")
	if err != nil {
		return fmt.Errorf("error collecting logs from log-collector: %v", err)
	}
	log.Printf("[%d] log files collected for pods", len(pods))

	services, err := cr.CollectServiceLogs("*")
	if err != nil {
		return fmt.Errorf("error collecting logs from log-collector: %v", err)
	}
	log.Printf("[%d] log files collected for services", len(services))
	return nil
}

func cleanUpLogging() error {
	if err := cr.Cleanup(); err != nil {
		return fmt.Errorf("error cleaning-up log-collector: %v", err)
	}
	return nil
}

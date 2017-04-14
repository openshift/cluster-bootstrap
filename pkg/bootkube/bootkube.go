package bootkube

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"github.com/kubernetes-incubator/bootkube/pkg/util/etcdutil"
)

const (
	assetTimeout    = 20 * time.Minute
	insecureAPIAddr = "http://127.0.0.1:8080"
)

var requiredPods = []string{
	"pod-checkpointer",
	"kube-apiserver",
	"kube-scheduler",
	"kube-controller-manager",
}

type Config struct {
	AssetDir        string
	PodManifestPath string
}

type bootkube struct {
	selfHostedEtcd  bool
	podManifestPath string
	assetDir        string
}

func NewBootkube(config Config) (*bootkube, error) {
	return &bootkube{
		assetDir:        config.AssetDir,
		podManifestPath: config.PodManifestPath,
		selfHostedEtcd:  fileutil.Exist(filepath.Join(config.AssetDir, asset.AssetPathBootstrapEtcd)),
	}, nil
}

func (b *bootkube) Run() error {
	defer func() {
		// Always clean up the bootstrap control plane and secrets.
		if err := CleanupBootstrapControlPlane(b.assetDir, b.podManifestPath); err != nil {
			UserOutput("Error cleaning up temporary bootstrap control plane: %v\n", err)
		}
	}()

	var err error
	defer func() {
		// Always report errors.
		if err != nil {
			UserOutput("Error: %v\n", err)
		}
	}()

	if err = CreateBootstrapControlPlane(b.assetDir, b.podManifestPath); err != nil {
		return err
	}

	if err = CreateAssets(filepath.Join(b.assetDir, asset.AssetPathManifests), assetTimeout); err != nil {
		return err
	}

	if b.selfHostedEtcd {
		requiredPods = append(requiredPods, "etcd-operator")
	}

	if err = WaitUntilPodsRunning(requiredPods, assetTimeout); err != nil {
		return err
	}

	if b.selfHostedEtcd {
		UserOutput("Migrating to self-hosted etcd cluster...\n")
		var etcdServiceIP string
		etcdServiceIP, err = detectEtcdIP(b.assetDir)
		if err != nil {
			return err
		}
		if err = etcdutil.Migrate(insecureAPIAddr, etcdServiceIP); err != nil {
			return err
		}
	}

	return nil
}

// All bootkube printing to stdout should go through this fmt.Printf wrapper.
// The stdout of bootkube should convey information useful to a human sitting
// at a terminal watching their cluster bootstrap itself. Otherwise the message
// should go to stderr.
func UserOutput(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

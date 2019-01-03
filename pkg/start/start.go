package start

import (
	"fmt"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"
)

const assetTimeout = 20 * time.Minute

type Config struct {
	AssetDir        string
	PodManifestPath string
	Strict          bool
	RequiredPods    []string
}

type startCommand struct {
	podManifestPath string
	assetDir        string
	requiredPods    []string
}

func NewStartCommand(config Config) (*startCommand, error) {
	return &startCommand{
		assetDir:        config.AssetDir,
		podManifestPath: config.PodManifestPath,
		requiredPods:    config.RequiredPods,
	}, nil
}

func (b *startCommand) Run() error {
	// TODO(diegs): create and share a single client rather than the kubeconfig once all uses of it
	// are migrated to client-go.
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: filepath.Join(b.assetDir, AssetPathAdminKubeConfig)},
		&clientcmd.ConfigOverrides{})

	bcp := NewBootstrapControlPlane(b.assetDir, b.podManifestPath)

	defer func() {
		// Always tear down the bootstrap control plane and clean up manifests and secrets.
		if err := bcp.Teardown(); err != nil {
			UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}()

	var err error
	defer func() {
		// Always report errors.
		if err != nil {
			UserOutput("Error: %v\n", err)
		}
	}()

	if err = bcp.Start(); err != nil {
		return err
	}

	if err = CreateAssets(kubeConfig, filepath.Join(b.assetDir, AssetPathManifests), assetTimeout); err != nil {
		return err
	}

	if err = WaitUntilPodsRunning(kubeConfig, b.requiredPods, assetTimeout); err != nil {
		return err
	}

	return nil
}

// All start command printing to stdout should go through this fmt.Printf wrapper.
// The stdout of the start command should convey information useful to a human sitting
// at a terminal watching their cluster bootstrap itself. Otherwise the message
// should go to stderr.
func UserOutput(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

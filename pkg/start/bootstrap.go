package start

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/openshift/cluster-bootstrap/pkg/common"
)

type bootstrapControlPlane struct {
	assetDir        string
	podManifestPath string
	ownedManifests  []string
	kubeApiHost     string
}

// newBootstrapControlPlane constructs a new bootstrap control plane object.
func newBootstrapControlPlane(assetDir, podManifestPath string, kubeApiHost string) *bootstrapControlPlane {
	return &bootstrapControlPlane{
		assetDir:        assetDir,
		podManifestPath: podManifestPath,
		kubeApiHost:     kubeApiHost,
	}
}

// Start seeds static manifests to the kubelet to launch the bootstrap control plane.
// Users should always ensure that Cleanup() is called even in the case of errors.
func (b *bootstrapControlPlane) Start() error {
	common.UserOutput("Starting temporary bootstrap control plane...\n")
	// Make secrets temporarily available to bootstrap cluster.
	if err := os.RemoveAll(bootstrapSecretsDir); err != nil {
		return err
	}
	secretsDir := filepath.Join(b.assetDir, assetPathSecrets)
	if _, err := copyDirectory(secretsDir, bootstrapSecretsDir, true /* overwrite */); err != nil {
		return err
	}
	// Copy the admin kubeconfig. TODO(diegs): this is kind of a hack, maybe do something better.
	if err := copyFile(filepath.Join(b.assetDir, assetPathAdminKubeConfig), filepath.Join(bootstrapSecretsDir, "kubeconfig"), true /* overwrite */); err != nil {
		return err
	}

	// Copy the static manifests to the kubelet's pod manifest path.
	manifestsDir := filepath.Join(b.assetDir, assetPathBootstrapManifests)
	ownedManifests, err := copyDirectory(manifestsDir, b.podManifestPath, false /* overwrite */)
	b.ownedManifests = ownedManifests // always copy in case of partial failure.
	if err != nil {
		return err
	}

	// Wait for kube-apiserver to be available and return.
	return b.waitForApi()
}

// waitForApi will wait until kube-apiserver readyz endpoint is available
func (b *bootstrapControlPlane) waitForApi() error {
	common.UserOutput("Waiting up to %v for the Kubernetes API\n", bootstrapPodsRunningTimeout)
	apiContext, cancel := context.WithTimeout(context.Background(), bootstrapPodsRunningTimeout)
	defer cancel()
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Transport: customTransport}
	previousErrorSuffix := ""
	wait.Until(func() {
		_, err := client.Get(fmt.Sprintf("https://%s/readyz", b.kubeApiHost))
		if err == nil {
			common.UserOutput("API is up\n")
			cancel()
		} else {
			chunks := strings.Split(err.Error(), ":")
			errorSuffix := chunks[len(chunks)-1]
			if previousErrorSuffix != errorSuffix {
				common.UserOutput("Still waiting for the Kubernetes API: %v\n", err)
				previousErrorSuffix = errorSuffix
			}
		}
	}, time.Second, apiContext.Done())
	if apiContext.Err() == context.Canceled {
		return nil
	} else {
		return fmt.Errorf("time out waiting for Kubernetes API")
	}
}

// Teardown brings down the bootstrap control plane and cleans up the temporary manifests and
// secrets. This function is idempotent.
func (b *bootstrapControlPlane) Teardown() error {
	if b == nil {
		return nil
	}

	common.UserOutput("Tearing down temporary bootstrap control plane...\n")
	if err := os.RemoveAll(bootstrapSecretsDir); err != nil {
		return err
	}
	for _, manifest := range b.ownedManifests {
		if err := os.Remove(manifest); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	b.ownedManifests = nil
	return nil
}

// copyFile copies a single file from src to dst. Returns an error if overwrite is true and dst
// exists, or if any I/O error occurs during copying.
func copyFile(src, dst string, overwrite bool) error {
	flags := os.O_CREATE | os.O_WRONLY
	if !overwrite {
		flags |= os.O_EXCL
	}

	dstfile, err := os.OpenFile(dst, flags, os.FileMode(0600))
	if err != nil {
		return err
	}
	defer dstfile.Close()

	srcfile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	_, err = io.Copy(dstfile, srcfile)
	return err
}

// copyDirectory copies srcDir to dstDir recursively. It returns the paths of files (not
// directories) that were copied.
func copyDirectory(srcDir, dstDir string, overwrite bool) ([]string, error) {
	var copied []string
	return copied, filepath.Walk(srcDir, func(src string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, strings.TrimPrefix(src, srcDir))
		if info.IsDir() {
			err = os.Mkdir(dst, os.FileMode(0700))
			if os.IsExist(err) {
				err = nil
			}
			return err
		}
		if err := copyFile(src, dst, overwrite); err != nil {
			return err
		}
		copied = append(copied, dst)
		return nil
	})
}

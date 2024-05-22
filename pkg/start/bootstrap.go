package start

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type bootstrapControlPlane struct {
	assetDir        string
	podManifestPath string
	ownedManifests  []string
	kubeApiHost     string

	// true if the control plane is a single node topology
	isSingleNodeToology bool

	// this client uses the localhost address to access the kube-apiserver
	loopbackClient kubernetes.Interface
}

// newBootstrapControlPlane constructs a new bootstrap control plane object.
func newBootstrapControlPlane(loopbackClient kubernetes.Interface, assetDir, podManifestPath string, kubeApiHost string, isSingleNodeToology bool) *bootstrapControlPlane {
	return &bootstrapControlPlane{
		loopbackClient:      loopbackClient,
		assetDir:            assetDir,
		podManifestPath:     podManifestPath,
		kubeApiHost:         kubeApiHost,
		isSingleNodeToology: isSingleNodeToology,
	}
}

// Start seeds static manifests to the kubelet to launch the bootstrap control plane.
// Users should always ensure that Cleanup() is called even in the case of errors.
func (b *bootstrapControlPlane) Start() error {
	UserOutput("Starting temporary bootstrap control plane...\n")
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
	UserOutput("Copying static manifests from: %s to: %s\n", manifestsDir, b.podManifestPath)
	ownedManifests, err := copyDirectory(manifestsDir, b.podManifestPath, false /* overwrite */)
	b.ownedManifests = ownedManifests // always copy in case of partial failure.
	if err != nil {
		return err
	}

	UserOutput("Successfully copied static pod manifests: %v\n", b.ownedManifests)

	// Wait for kube-apiserver to be available and return.
	return b.waitForApi()
}

// waitForApi will wait until kube-apiserver readyz endpoint is available
func (b *bootstrapControlPlane) waitForApi() error {
	UserOutput("Waiting up to %v for the Kubernetes API\n", bootstrapPodsRunningTimeout)
	apiContext, cancel := context.WithTimeout(context.Background(), bootstrapPodsRunningTimeout)
	defer cancel()
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Transport: customTransport}
	previousError := ""
	err := wait.PollUntil(time.Second, func() (bool, error) {
		if _, err := client.Get(fmt.Sprintf("https://%s/readyz", b.kubeApiHost)); err == nil {
			UserOutput("API is up\n")
			return true, nil
		} else if previousError != err.Error() {
			UserOutput("Still waiting for the Kubernetes API: %v \n", err)
			previousError = err.Error()
		}

		return false, nil
	}, apiContext.Done())
	if err != nil {
		return fmt.Errorf("time out waiting for Kubernetes API")
	}

	return nil
}

// waitForTermination will wait until kube-apiserver /version endpoint returns an error.
func (b *bootstrapControlPlane) waitForTermination(timeout time.Duration) error {
	if timeout == 0 {
		return nil
	}
	UserOutput("Waiting up to %v for the Kubernetes API to terminate\n", timeout)
	apiContext, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Transport: customTransport}
	previousError := ""
	err := wait.PollUntil(time.Second, func() (bool, error) {
		if _, err := client.Get(fmt.Sprintf("https://%s/version", b.kubeApiHost)); err == nil {
			return false, nil
		} else if net.IsConnectionRefused(err) {
			UserOutput("Kubernetes API has terminated.\n")
			return true, nil
		} else if previousError != err.Error() {
			UserOutput("Still waiting for the Kubernetes API to terminate: %v \n", err)
			previousError = err.Error()
		}

		return false, nil
	}, apiContext.Done())
	if err != nil {
		return fmt.Errorf("time out waiting for Kubernetes API to terminate")
	}

	return nil
}

func (b *bootstrapControlPlane) waitForAtLeastTwoAPIServerAvailable(timeout time.Duration) error {
	UserOutput("Waiting up to %v for at least two kube-apiserver instances to be available\n", timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	lastMsg := ""
	err := wait.PollUntil(5*time.Second, func() (bool, error) {
		addresses, err := getKubeAPIEndpoints(context.Background(), b.loopbackClient)
		if err != nil {
			msg := fmt.Sprintf("got an unexpected error: %v, will continue to retry\n", err)
			if msg != lastMsg {
				UserOutput(msg)
			}

			lastMsg = msg
			return false, nil
		}
		if len(addresses) <= 1 {
			msg := fmt.Sprintf("still waiting for at least two kube-apiserver instances, seen: %v\n", addresses)
			if msg != lastMsg {
				UserOutput(msg)
			}

			lastMsg = msg
			return false, nil
		}

		UserOutput("We have at least two kube-apiserver instances available: %v\n", addresses)
		return true, nil
	}, ctx.Done())
	if err != nil {
		return fmt.Errorf("time out waiting for at least two kube-apiserver instances to be available: %w", err)
	}

	return nil
}

func getKubeAPIEndpoints(ctx context.Context, client kubernetes.Interface) ([]string, error) {
	endpoint, err := client.CoreV1().Endpoints("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list endpoints for default/kubernetes: %w", err)
	}
	addresses := []string{}
	for _, subset := range endpoint.Subsets {
		for _, address := range subset.Addresses {
			addresses = append(addresses, address.IP)
		}
	}
	return addresses, nil
}

// Teardown brings down the bootstrap control plane and cleans up the temporary manifests and
// secrets. This function is idempotent.
func (b *bootstrapControlPlane) Teardown(terminationTimeout time.Duration) error {
	if b == nil {
		return nil
	}

	if err := b.teardownBootstrapAPIServer(); err != nil {
		return fmt.Errorf("error occurred while tearing down bootstrap kube-apiserver: %w", err)
	}

	UserOutput("Tearing down temporary bootstrap control plane...\n")
	if err := os.RemoveAll(bootstrapSecretsDir); err != nil {
		return err
	}
	for _, manifest := range b.ownedManifests {
		if err := os.Remove(manifest); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	b.ownedManifests = nil

	if err := b.waitForTermination(terminationTimeout); err != nil {
		return fmt.Errorf("unexpected error while waiting for bootstrap API to terminate: %w", err)
	}
	return nil
}

func (b *bootstrapControlPlane) teardownBootstrapAPIServer() error {
	if b.isSingleNodeToology {
		// no effect of single node topology
		return nil
	}
	UserOutput("start waiting for at least 2 kube-apiserver instances to show up in default/kubernetes endpoint object\n")
	if err := b.waitForAtLeastTwoAPIServerAvailable(apiAvailabaleWaitTimeout); err != nil {
		return fmt.Errorf("error occurred while waiting for at least two available kube-apiserver instances: %w", err)
	}

	file := filepath.Join(b.podManifestPath, "kube-apiserver-pod.yaml")
	UserOutput("removing the static pod yaml for bootstrap kube-apiserer: %s\n", file)
	if err := os.Remove(file); err != nil {
		if os.IsNotExist(err) {
			UserOutput("static pod yaml for bootstrap kube-apiserer does not exist, skipping teardown\n")
			return nil
		}
		return fmt.Errorf("error occurred while removing bootstrap kube-apiserer static pod yaml: %w", err)
	}
	UserOutput(fmt.Sprintf("successfully removed bootstrap kube-apiserer static pod yaml: %s\n", file))

	UserOutput("wait for bootstrap kube-apiserver to gracefully shutdown\n")
	// TODO: make it configurable
	// alternative: <-time.After(100 * time.Second)
	if err := b.waitForTermination(3 * time.Minute); err != nil {
		return fmt.Errorf("error occurred while waiting for bootstrap apiserver to terminate: %w", err)
	}
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

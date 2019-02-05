package start

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// how long we wait until the bootstrap pods to be running
	bootstrapPodsRunningTimeout = 20 * time.Minute
)

type Config struct {
	AssetDir             string
	PodManifestPath      string
	Strict               bool
	RequiredPodPrefixes  map[string][]string
	WaitForTearDownEvent string
}

type startCommand struct {
	podManifestPath      string
	assetDir             string
	strict               bool
	requiredPodPrefixes  map[string][]string
	waitForTearDownEvent string
}

func NewStartCommand(config Config) (*startCommand, error) {
	return &startCommand{
		assetDir:             config.AssetDir,
		podManifestPath:      config.PodManifestPath,
		strict:               config.Strict,
		requiredPodPrefixes:  config.RequiredPodPrefixes,
		waitForTearDownEvent: config.WaitForTearDownEvent,
	}, nil
}

func (b *startCommand) Run() error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", filepath.Join(b.assetDir, assetPathAdminKubeConfig))
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	bcp := newBootstrapControlPlane(b.assetDir, b.podManifestPath)

	defer func() {
		// Always tear down the bootstrap control plane and clean up manifests and secrets.
		if err := bcp.Teardown(); err != nil {
			UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}()

	defer func() {
		// Always report errors.
		if err != nil {
			UserOutput("Error: %v\n", err)
		}
	}()

	if err = bcp.Start(); err != nil {
		return err
	}

	if err = createAssets(restConfig, filepath.Join(b.assetDir, assetPathManifests), bootstrapPodsRunningTimeout, b.strict); err != nil {
		return err
	}

	if err = waitUntilPodsRunning(client, b.requiredPodPrefixes, bootstrapPodsRunningTimeout); err != nil {
		return err
	}

	// notify installer that we are ready to tear down the temporary bootstrap control plane
	UserOutput("Sending bootstrap-success event.")
	if _, err := client.CoreV1().Events("kube-system").Create(makeBootstrapSuccessEvent("kube-system", "bootstrap-success")); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// optionally wait for tear down event coming from the installer. This is necessary to
	// remove the bootstrap node from the AWS load balancer.
	if len(b.waitForTearDownEvent) != 0 {
		ss := strings.Split(b.waitForTearDownEvent, "/")
		if len(ss) != 2 {
			return fmt.Errorf("tear down event name of format <namespace>/<event-name> expected, got: %q", b.waitForTearDownEvent)
		}
		ns, name := ss[0], ss[1]
		if err := waitForEvent(context.TODO(), client, ns, name); err != nil {
			return err
		}
		UserOutput("Got %s event.", b.waitForTearDownEvent)
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

func waitForEvent(ctx context.Context, client kubernetes.Interface, ns, name string) error {
	return wait.PollImmediateUntil(time.Second, func() (done bool, err error) {
		if _, err := client.CoreV1().Events(ns).Get(name, metav1.GetOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			UserOutput("Error waiting for %s/%s event: %v", ns, name, err)
			return false, nil
		}
		return true, nil
	}, ctx.Done())
}

func makeBootstrapSuccessEvent(ns, name string) *corev1.Event {
	currentTime := metav1.Time{Time: time.Now()}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		InvolvedObject: corev1.ObjectReference{
			Namespace: ns,
		},
		Message:        "Required control plane pods have been created",
		Count:          1,
		FirstTimestamp: currentTime,
		LastTimestamp:  currentTime,
	}
	return event
}

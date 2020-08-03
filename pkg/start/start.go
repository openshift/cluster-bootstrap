package start

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// how long we wait until the bootstrap pods to be running
	bootstrapPodsRunningTimeout = 20 * time.Minute
	// how long we wait until the assets must all be created
	assetsCreatedTimeout = 60 * time.Minute
)

type Config struct {
	AssetDir             string
	PodManifestPath      string
	Strict               bool
	RequiredPodPrefixes  map[string][]string
	WaitForTearDownEvent string
	EarlyTearDown        bool
}

type startCommand struct {
	podManifestPath      string
	assetDir             string
	strict               bool
	requiredPodPrefixes  map[string][]string
	waitForTearDownEvent string
	earlyTearDown        bool
}

func NewStartCommand(config Config) (*startCommand, error) {
	return &startCommand{
		assetDir:             config.AssetDir,
		podManifestPath:      config.PodManifestPath,
		strict:               config.Strict,
		requiredPodPrefixes:  config.RequiredPodPrefixes,
		waitForTearDownEvent: config.WaitForTearDownEvent,
		earlyTearDown:        config.EarlyTearDown,
	}, nil
}

func (b *startCommand) Run() error {

	bcp := newBootstrapControlPlane(b.assetDir, b.podManifestPath)

	if err := bcp.Start(); err != nil {
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

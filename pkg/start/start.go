package start

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openshift/library-go/pkg/assets/create"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	EarlyTearDown        bool
	AssetsCreatedTimeout time.Duration
}

type startCommand struct {
	podManifestPath      string
	assetDir             string
	strict               bool
	requiredPodPrefixes  map[string][]string
	waitForTearDownEvent string
	earlyTearDown        bool
	assetsCreatedTimeout time.Duration
}

func NewStartCommand(config Config) (*startCommand, error) {
	return &startCommand{
		assetDir:             config.AssetDir,
		podManifestPath:      config.PodManifestPath,
		strict:               config.Strict,
		requiredPodPrefixes:  config.RequiredPodPrefixes,
		waitForTearDownEvent: config.WaitForTearDownEvent,
		earlyTearDown:        config.EarlyTearDown,
		assetsCreatedTimeout: config.AssetsCreatedTimeout,
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

	bcp := newBootstrapControlPlane(client, b.assetDir, b.podManifestPath)

	// Always tear down the bootstrap control plane and clean up manifests and secrets.
	defer func() {
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

	// We don't want the client contact the API servers via load-balancer, but only talk to the local API server.
	// This will speed up the initial "where is working API server" process.
	localClientConfig := rest.CopyConfig(restConfig)
	localClientConfig.Host = "localhost:6443"
	// Set the ServerName to original hostname so we pass the certificate check.
	hostURL, err := url.Parse(restConfig.Host)
	if err != nil {
		return err
	}
	localClientConfig.ServerName, _, err = net.SplitHostPort(hostURL.Host)
	if err != nil {
		return err
	}

	// create assets against localhost apiserver (in the background) and wait for control plane to be up
	createAssetsInBackground := func(ctx context.Context, cancel func(), client *rest.Config) *sync.WaitGroup {
		done := sync.WaitGroup{}
		done.Add(1)
		go func() {
			defer done.Done()
			if err := create.EnsureManifestsCreated(ctx, filepath.Join(b.assetDir, assetPathManifests), client, create.CreateOptions{
				Verbose: true,
				StdErr:  os.Stderr,
			}); err != nil {
				select {
				case <-ctx.Done():
				default:
					UserOutput("Assert creation failed: %v\n", err)
					cancel()
				}
			}
		}()
		return &done
	}
	ctx, cancel := context.WithTimeout(context.TODO(), bootstrapPodsRunningTimeout)
	defer cancel()
	assetsDone := createAssetsInBackground(ctx, cancel, localClientConfig)
	if err = waitUntilPodsRunning(ctx, client, b.requiredPodPrefixes); err != nil {
		return err
	}
	cancel()
	assetsDone.Wait()

	// notify installer that we are ready to tear down the temporary bootstrap control plane
	UserOutput("Sending bootstrap-success event.")
	if _, err := client.CoreV1().Events("kube-system").Create(makeBootstrapSuccessEvent("kube-system", "bootstrap-success")); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// continue with assets
	ctx, cancel = context.WithTimeout(context.Background(), b.assetsCreatedTimeout)
	defer cancel()
	if b.earlyTearDown {
		// switch over to ELB client and continue with the assets
		assetsDone = createAssetsInBackground(ctx, cancel, restConfig)
	} else {
		// we don't tear down the local control plane early. So we can keep using it and enjoy the speed up.
		assetsDone = createAssetsInBackground(ctx, cancel, localClientConfig)
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

	// tear down the bootstrap control plane. Set bcp to nil to avoid a second tear down in the defer func.
	if b.earlyTearDown {
		err = bcp.Teardown()
		bcp = nil
		if err != nil {
			UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}

	// wait for the tail of assets to be created after tear down
	UserOutput("Waiting for remaining assets to be created.\n")
	assetsDone.Wait()

	// We want to fail in case we failed to create some manifests
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timed out creating manifests")
	}
	UserOutput("Sending bootstrap-finished event.")
	if _, err := client.CoreV1().Events("kube-system").Create(makeBootstrapSuccessEvent("kube-system", "bootstrap-finished")); err != nil && !apierrors.IsAlreadyExists(err) {
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

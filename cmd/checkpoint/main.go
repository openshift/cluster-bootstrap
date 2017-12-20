package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubernetes-incubator/bootkube/pkg/checkpoint"
)

const (
	nodeNameEnv     = "NODE_NAME"
	podNameEnv      = "POD_NAME"
	podNamespaceEnv = "POD_NAMESPACE"

	defaultRuntimeEndpoint       = "unix:///var/run/dockershim.sock"
	defaultRuntimeRequestTimeout = 2 * time.Minute
	defaultCheckpointGracePeriod = 1 * time.Minute
)

var (
	lockfilePath          string
	kubeconfigPath        string
	remoteRuntimeEndpoint string
	runtimeRequestTimeout time.Duration
	checkpointGracePeriod time.Duration
)

func init() {
	flag.StringVar(&lockfilePath, "lock-file", "/var/run/lock/pod-checkpointer.lock", "The path to lock file for checkpointer to use")
	flag.StringVar(&kubeconfigPath, "kubeconfig", "/etc/kubernetes/kubeconfig", "Path to a kubeconfig file containing credentials used to talk to the kubelet.")
	flag.Set("logtostderr", "true")
	flag.StringVar(&remoteRuntimeEndpoint, "container-runtime-endpoint", defaultRuntimeEndpoint, "[Experimental] The endpoint of remote runtime service. Currently unix socket is supported on Linux, and tcp is supported on windows.  Examples:'unix:///var/run/dockershim.sock', 'tcp://localhost:3735'")
	flag.DurationVar(&runtimeRequestTimeout, "runtime-request-timeout", defaultRuntimeRequestTimeout, "Timeout of all runtime requests except long running request - pull, logs, exec and attach. When timeout exceeded, kubelet will cancel the request, throw out an error and retry later.")
	flag.DurationVar(&checkpointGracePeriod, "checkpoint-grace-period", defaultCheckpointGracePeriod, "Grace period for cleaning up checkpoints when the parent pod is deleted. Non-zero values are helpful for accommodating control plane eventual consistency.")
}

func main() {
	flag.Parse()
	defer glog.Flush()

	glog.Info("Determining environment from downward API")
	nodeName, podName, podNamespace, err := readDownwardAPI()
	if err != nil {
		glog.Fatalf("Error reading downward API: %v", err)
	}

	glog.Infof("Trying to acquire the flock at %q", lockfilePath)
	if err := flock(lockfilePath); err != nil {
		glog.Fatalf("Error when acquiring the flock: %v", err)
	}

	glog.Infof("Starting checkpointer for node: %s", nodeName)
	// This is run as a static pod, so we can't use InClusterConfig because
	// KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT won't be set in
	// the pod.
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		glog.Fatalf("Failed to load kubeconfig: %v", err)
	}
	if err := checkpoint.Run(checkpoint.Options{
		CheckpointerPod: checkpoint.CheckpointerPod{
			NodeName:     nodeName,
			PodName:      podName,
			PodNamespace: podNamespace,
		},
		KubeConfig:            kubeConfig,
		RemoteRuntimeEndpoint: remoteRuntimeEndpoint,
		RuntimeRequestTimeout: runtimeRequestTimeout,
		CheckpointGracePeriod: checkpointGracePeriod,
	}); err != nil {
		glog.Fatalf("Error starting checkpointer: %v", err)
	}
}

// flock tries to grab a flock on the given path.
// If the lock is already acquired by other process, the function will block.
// TODO(yifan): Maybe replace this with kubernetes/pkg/util/flock.Acquire() once
// https://github.com/kubernetes/kubernetes/issues/42929 is solved, or maybe not.
func flock(path string) error {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR, 0600)
	if err != nil {
		return err
	}

	// We don't need to close the fd since we should hold
	// it until the process exits.

	return syscall.Flock(fd, syscall.LOCK_EX)
}

// readDownwardAPI fills the node name, pod name, and pod namespace.
func readDownwardAPI() (nodeName, podName, podNamespace string, err error) {
	nodeName = os.Getenv(nodeNameEnv)
	if nodeName == "" {
		return "", "", "", fmt.Errorf("missing required environment variable: %s", nodeNameEnv)
	}
	podName = os.Getenv(podNameEnv)
	if podName == "" {
		return "", "", "", fmt.Errorf("missing required environment variable: %s", podNameEnv)
	}
	podNamespace = os.Getenv(podNamespaceEnv)
	if podNamespace == "" {
		return "", "", "", fmt.Errorf("missing required environment variable: %s", podNamespaceEnv)
	}
	return nodeName, podName, podNamespace, nil
}

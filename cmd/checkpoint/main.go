package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	nodeNameEnv     = "NODE_NAME"
	podNameEnv      = "POD_NAME"
	podNamespaceEnv = "POD_NAMESPACE"

	activeCheckpointPath    = "/etc/kubernetes/manifests"
	inactiveCheckpointPath  = "/etc/kubernetes/inactive-manifests"
	checkpointSecretPath    = "/etc/kubernetes/checkpoint-secrets"
	checkpointConfigMapPath = "/etc/kubernetes/checkpoint-configmaps"

	shouldCheckpointAnnotation = "checkpointer.alpha.coreos.com/checkpoint"    // = "true"
	checkpointParentAnnotation = "checkpointer.alpha.coreos.com/checkpoint-of" // = "podName"
	podSourceAnnotation        = "kubernetes.io/config.source"

	shouldCheckpoint = "true"
	podSourceFile    = "file"

	defaultRuntimeEndpoint       = "unix:///var/run/dockershim.sock"
	defaultRuntimeRequestTimeout = 2 * time.Minute
	defaultPollingFrequency      = 3 * time.Second
	defaultCheckpointTimeout     = 1 * time.Minute
)

var (
	pollingFrequency time.Duration
	lockfilePath     string

	// TODO(yifan): Put these into a struct when necessary.
	nodeName              string
	podName               string
	podNamespace          string
	kubeconfigPath        string
	remoteRuntimeEndpoint string
	runtimeRequestTimeout time.Duration
	lastCheckpoint        time.Time
	checkPointTimeout     time.Duration

	// podSerializer is an encoder for writing checkpointed pods.
	//
	// Perfer this instead of json.Marshal because it corrects metadata before
	// serializing. For example it automatically fills in the "apiVersion" field.
	podSerializer = scheme.Codecs.EncoderForVersion(
		json.NewSerializer(
			json.DefaultMetaFactory,
			scheme.Scheme, // client-go's default scheme.
			scheme.Scheme,
			false, // don't pretty print.
		),
		v1.SchemeGroupVersion,
	)
)

func init() {
	flag.StringVar(&lockfilePath, "lock-file", "/var/run/lock/pod-checkpointer.lock", "The path to lock file for checkpointer to use")
	flag.StringVar(&kubeconfigPath, "kubeconfig", "/etc/kubernetes/kubeconfig", "Path to a kubeconfig file containing credentials used to talk to the kubelet.")
	flag.Set("logtostderr", "true")
	flag.StringVar(&remoteRuntimeEndpoint, "container-runtime-endpoint", defaultRuntimeEndpoint, "[Experimental] The endpoint of remote runtime service. Currently unix socket is supported on Linux, and tcp is supported on windows.  Examples:'unix:///var/run/dockershim.sock', 'tcp://localhost:3735'")
	flag.DurationVar(&runtimeRequestTimeout, "runtime-request-timeout", defaultRuntimeRequestTimeout, "Timeout of all runtime requests except long running request - pull, logs, exec and attach. When timeout exceeded, kubelet will cancel the request, throw out an error and retry later.")
	flag.DurationVar(&pollingFrequency, "polling-frequency", defaultPollingFrequency, "Rate at which the kubelet and CRI shim is polled for running pods information")
	flag.DurationVar(&checkPointTimeout, "api-poll-timeout", defaultCheckpointTimeout, "Rate at which the API server is polled for changes to secrets and configmaps")
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
func readDownwardAPI() {
	nodeName = os.Getenv(nodeNameEnv)
	if nodeName == "" {
		glog.Fatalf("Missing required environment variable: %s", nodeNameEnv)
	}
	podName = os.Getenv(podNameEnv)
	if podName == "" {
		glog.Fatalf("Missing required environment variable: %s", podNameEnv)
	}
	podNamespace = os.Getenv(podNamespaceEnv)
	if podNamespace == "" {
		glog.Fatalf("Missing required environment variable: %s", podNamespaceEnv)
	}
}

func main() {
	flag.Parse()
	defer glog.Flush()

	readDownwardAPI()

	glog.Infof("Trying to acquire the flock at %q", lockfilePath)

	if err := flock(lockfilePath); err != nil {
		glog.Fatalf("Error when acquiring the flock: %v", err)
	}

	glog.Infof("Starting checkpointer for node: %s", nodeName)
	// This is run as a static pod, so we can't use InClusterConfig because
	// KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT won't be set in
	// the pod.
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		glog.Fatalf("Failed to load kubeconfig: %v", err)
	}
	client := kubernetes.NewForConfigOrDie(kubeConfig)
	kubelet, err := newKubeletClient(kubeConfig)
	if err != nil {
		glog.Fatalf("Failed to load kubelet client: %v", err)
	}

	run(client, kubelet)
}

func run(client kubernetes.Interface, kubelet *kubeletClient) {
	for {
		time.Sleep(pollingFrequency)

		// We must use both the :10255/pods endpoint and CRI shim, because /pods
		// endpoint could have stale data. The /pods endpoint will only show the last cached
		// status which has successfully been written to an apiserver. However, if there is
		// no apiserver, we may get stale state (e.g. saying pod is running, when it really is
		// not).
		localParentPods := kubelet.localParentPods()
		localRunningPods := kubelet.localRunningPods()

		createCheckpointsForValidParents(client, localParentPods)

		// Try to get scheduled pods from the apiserver.
		// These will be used to GC checkpoints for parents no longer scheduled to this node.
		// A return value of nil is assumed to be "could not contact apiserver"
		// TODO(aaron): only check this every 30 seconds or so
		apiParentPods := getAPIParentPods(client, nodeName)

		// Get on disk copies of (in)active checkpoints
		//TODO(aaron): Could be racy to load from disk each time, but much easier than trying to keep in-memory state in sync.
		activeCheckpoints := getFileCheckpoints(activeCheckpointPath)
		inactiveCheckpoints := getFileCheckpoints(inactiveCheckpointPath)

		start, stop, remove := process(localRunningPods, localParentPods, apiParentPods, activeCheckpoints, inactiveCheckpoints)

		// Handle remove at last because we may still have some work to do
		// before removing the checkpointer itself.
		handleStop(stop)
		handleStart(start)
		handleRemove(remove)
	}
}

// process() makes decisions on which checkpoints need to be started, stopped, or removed.
// It makes this decision based on inspecting the states from kubelet, apiserver, active/inactive checkpoints.
//
// - localRunningPods: running pods retrieved from CRI shim. Minimal amount of info (no podStatus) as it is extracted from container runtime.
// - localParentPods: pod state from kubelet api for all "to be checkpointed" pods - podStatus may be stale (only as recent as last apiserver contact)
// - apiParentPods: pod state from the api server for all "to be checkpointed" pods
// - activeCheckpoints: checkpoint pod manifests which are currently active & in the static pod manifest
// - inactiveCheckpoints: checkpoint pod manifest which are stored in an inactive directory, but are ready to be activated
//
// The return values are checkpoints which should be started or stopped, and checkpoints which need to be removed altogether.
// The removal of a checkpoint means its parent is no longer scheduled to this node, and we need to GC active / inactive
// checkpoints as well as any secrets / configMaps which are no longer necessary.
func process(localRunningPods, localParentPods, apiParentPods, activeCheckpoints, inactiveCheckpoints map[string]*v1.Pod) (start, stop, remove []string) {
	// If this variable is filled, then it means we need to remove the pod-checkpointer's checkpoint.
	// We treat the pod-checkpointer's checkpoint specially because we want to put it at the end of
	// the remove queue.
	var podCheckpointerID string

	// We can only make some GC decisions if we've successfully contacted an apiserver.
	// When apiParentPods == nil, that means we were not able to get an updated list of pods.
	removeMap := make(map[string]struct{})
	if len(apiParentPods) != 0 {

		// Scan for inacive checkpoints we should GC
		for id := range inactiveCheckpoints {
			// If the inactive checkpoint still has a parent pod, do nothing.
			// This means the kubelet thinks it should still be running, which has the same scheduling info that we do --
			// so we won't make any decisions about its checkpoint.
			// TODO(aaron): This is a safety check, and may not be necessary -- question is do we trust that the api state we received
			//              is accurate -- and that we should ignore our local state (or assume it could be inaccurate). For example,
			//              local kubelet pod state will be innacurate in the case that we can't contact apiserver (kubelet only keeps
			//              cached responses from api) -- however, we're assuming we've been able to contact api, so this likely is moot.
			if _, ok := localParentPods[id]; ok {
				glog.V(4).Infof("API GC: skipping inactive checkpoint %s", id)
				continue
			}

			// If the inactive checkpoint does not have a parent in the api-server, we must assume it should no longer be running on this node.
			// NOTE: It's possible that a replacement for this pod has not been rescheduled elsewhere, but that's not something we can base our decision on.
			//       For example, if a single scheduler is running, and the node is drained, the scheduler pod will be deleted and there will be no replacement.
			//       However, we don't know this, and as far as the checkpointer is concerned - that pod is no longer scheduled to this node.
			if _, ok := apiParentPods[id]; !ok {
				glog.V(4).Infof("API GC: should remove inactive checkpoint %s", id)

				removeMap[id] = struct{}{}
				if isPodCheckpointer(inactiveCheckpoints[id]) {
					podCheckpointerID = id
					break
				}

				delete(inactiveCheckpoints, id)
			}
		}

		// Scan active checkpoints we should GC
		for id := range activeCheckpoints {
			// If the active checkpoint does not have a parent in the api-server, we must assume it should no longer be running on this node.
			if _, ok := apiParentPods[id]; !ok {
				glog.V(4).Infof("API GC: should remove active checkpoint %s", id)

				removeMap[id] = struct{}{}
				if isPodCheckpointer(activeCheckpoints[id]) {
					podCheckpointerID = id
					break
				}

				delete(activeCheckpoints, id)
			}
		}
	}

	// Remove all checkpoints if we need to remove the pod checkpointer itself.
	if podCheckpointerID != "" {
		glog.V(4).Info("Pod checkpointer is removed, removing all checkpoints")
		for id := range inactiveCheckpoints {
			removeMap[id] = struct{}{}
			delete(inactiveCheckpoints, id)
		}
		for id := range activeCheckpoints {
			removeMap[id] = struct{}{}
			delete(activeCheckpoints, id)
		}
	}

	// Can make decisions about starting/stopping checkpoints just with local state.
	//
	// If there is an inactive checkpoint, and no parent pod is running, or the checkpoint
	// is the pod-checkpointer, start the checkpoint.
	for id := range inactiveCheckpoints {
		_, ok := localRunningPods[id]
		if !ok || isPodCheckpointer(inactiveCheckpoints[id]) {
			glog.V(4).Infof("Should start checkpoint %s", id)
			start = append(start, id)
		}
	}

	// If there is an active checkpoint and a running parent pod, stop the active checkpoint
	// unless this is the pod-checkpointer.
	// The parent may not be in a running state, but the kubelet is trying to start it
	// so we should get out of the way.
	for id := range activeCheckpoints {
		_, ok := localRunningPods[id]
		if ok && !isPodCheckpointer(activeCheckpoints[id]) {
			glog.V(4).Infof("Should stop checkpoint %s", id)
			stop = append(stop, id)
		}
	}

	// De-duped checkpoints to remove. If we decide to GC a checkpoint, we will clean up both inactive/active.
	for k := range removeMap {
		if k == podCheckpointerID {
			continue
		}
		remove = append(remove, k)
	}
	// Put pod checkpoint at the last of the queue.
	if podCheckpointerID != "" {
		remove = append(remove, podCheckpointerID)
	}

	return start, stop, remove
}

// createCheckpointsForValidParents will iterate through pods which are candidates for checkpointing, then:
// - checkpoint any remote assets they need (e.g. secrets, configmaps)
// - sanitize their podSpec, removing unnecessary information
// - store the manifest on disk in an "inactive" checkpoint location
//TODO(aaron): Add support for checkpointing configMaps
func createCheckpointsForValidParents(client kubernetes.Interface, pods map[string]*v1.Pod) {
	for _, pod := range pods {
		id := PodFullName(pod)

		cp, err := copyPod(pod)
		if err != nil {
			glog.Errorf("Failed to create checkpoint pod copy for %s: %v", id, err)
			continue
		}

		cp, err = sanitizeCheckpointPod(cp)
		if err != nil {
			glog.Errorf("Failed to sanitize manifest for %s: %v", id, err)
			continue
		}

		podChanged, err := writeCheckpointManifest(cp)
		if err != nil {
			glog.Errorf("Failed to write checkpoint for %s: %v", id, err)
			continue
		}

		// Check for secret and configmap changes if the pods have change or they haven't been checked in a while
		if podChanged || lastCheckpoint.IsZero() || time.Since(lastCheckpoint) >= checkPointTimeout {

			_, err = checkpointSecretVolumes(client, pod)
			if err != nil {
				//TODO(aaron): This can end up spamming logs at times when api-server is unavailable. To reduce spam
				//             we could only log error if api-server can't be contacted and existing secret doesn't exist.
				glog.Errorf("Failed to checkpoint secrets for pod %s: %v", id, err)
				continue
			}

			lastCheckpoint = time.Now()

			_, err = checkpointConfigMapVolumes(client, pod)
			if err != nil {
				//TODO(aaron): This can end up spamming logs at times when api-server is unavailable. To reduce spam
				//             we could only log error if api-server can't be contacted and existing configmap doesn't exist.
				glog.Errorf("Failed to checkpoint configMaps for pod %s: %v", id, err)
				continue
			}
		}
	}
}

// writeCheckpointManifest will save the pod to the inactive checkpoint location if it doesn't already exist.
func writeCheckpointManifest(pod *v1.Pod) (bool, error) {
	buff := &bytes.Buffer{}
	if err := podSerializer.Encode(pod, buff); err != nil {
		return false, err
	}
	path := filepath.Join(inactiveCheckpointPath, pod.Namespace+"-"+pod.Name+".json")
	// Make sure the inactive checkpoint path exists.
	if err := os.MkdirAll(filepath.Dir(path), 0600); err != nil {
		return false, err
	}
	return writeManifestIfDifferent(path, PodFullName(pod), buff.Bytes())
}

// writeManifestIfDifferent writes `data` to `path` if data is different from the existing content.
// The `name` parameter is used for debug output.
func writeManifestIfDifferent(path, name string, data []byte) (bool, error) {
	existing, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if bytes.Equal(existing, data) {
		glog.V(4).Infof("Checkpoint manifest for %q already exists. Skipping", name)
		return false, nil
	}
	glog.Infof("Writing manifest for %q to %q", name, path)
	return true, writeAndAtomicRename(path, data, 0644)
}

// isPodCheckpointer returns true if the manifest is the pod checkpointer (has the same name as the parent).
// For example, the pod.Name would be "pod-checkpointer".
// The podName would be "pod-checkpointer" or "pod-checkpointer-172.17.4.201" where
// "172.17.4.201" is the nodeName.
func isPodCheckpointer(pod *v1.Pod) bool {
	if pod.Namespace != podNamespace {
		return false
	}
	return pod.Name == strings.TrimSuffix(podName, "-"+nodeName)
}

func sanitizeCheckpointPod(cp *v1.Pod) (*v1.Pod, error) {
	trueVar := true

	// Keep same name, namespace, and labels as parent.
	cp.ObjectMeta = metav1.ObjectMeta{
		Name:        cp.Name,
		Namespace:   cp.Namespace,
		Annotations: make(map[string]string),
		Labels:      cp.Labels,
		// Set the ownerRef to the parent pod. We do this because:
		// If the ownerRef stays the same (e.g. the original deployment), then the deployment controller will try to manage the static/mirror pod.
		// If we clear the ownerRef, then a higher-level object will adopt this pod based on the label selector (e.g. the original deployment).
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: cp.APIVersion,
				Kind:       cp.Kind,
				Name:       cp.Name,
				UID:        cp.UID,
				Controller: &trueVar,
			},
		},
	}

	// Track this checkpoint's parent pod
	cp.Annotations[checkpointParentAnnotation] = cp.Name

	// Remove Service Account
	cp.Spec.ServiceAccountName = ""
	cp.Spec.DeprecatedServiceAccount = ""

	// Sanitize the volumes
	for i := range cp.Spec.Volumes {
		v := &cp.Spec.Volumes[i]
		if v.Secret != nil {
			v.HostPath = &v1.HostPathVolumeSource{Path: secretPath(cp.Namespace, cp.Name, v.Secret.SecretName)}
			v.Secret = nil
		} else if v.ConfigMap != nil {
			v.HostPath = &v1.HostPathVolumeSource{Path: configMapPath(cp.Namespace, cp.Name, v.ConfigMap.Name)}
			v.ConfigMap = nil
		}
	}

	// Clear pod status
	cp.Status.Reset()

	return cp, nil
}

// getFileCheckpoints will retrieve all checkpoint manifests from a given filepath.
func getFileCheckpoints(path string) map[string]*v1.Pod {
	checkpoints := make(map[string]*v1.Pod)

	fi, err := ioutil.ReadDir(path)
	if err != nil {
		glog.Fatalf("Failed to read checkpoint manifest path: %v", err)
	}

	for _, f := range fi {
		manifest := filepath.Join(path, f.Name())
		b, err := ioutil.ReadFile(manifest)
		if err != nil {
			glog.Errorf("Error reading manifest: %v", err)
			continue
		}

		cp := &v1.Pod{}
		if err := runtime.DecodeInto(api.Codecs.UniversalDecoder(), b, cp); err != nil {
			glog.Errorf("Error unmarshalling manifest from %s: %v", filepath.Join(path, f.Name()), err)
			continue
		}

		if isCheckpoint(cp) {
			if _, ok := checkpoints[PodFullName(cp)]; ok { // sanity check
				glog.Warningf("Found multiple checkpoint pods in %s with same id: %s", path, PodFullName(cp))
			}
			checkpoints[PodFullName(cp)] = cp
		}
	}
	return checkpoints
}

// getAPIParentPods will retrieve all pods from apiserver that are parents & should be checkpointed
func getAPIParentPods(client kubernetes.Interface, nodeName string) map[string]*v1.Pod {
	opts := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(api.PodHostField, nodeName).String(),
	}

	podList, err := client.Core().Pods(api.NamespaceAll).List(opts)
	if err != nil {
		glog.Warningf("Unable to contact APIServer, skipping garbage collection: %v", err)
		return nil
	}
	return podListToParentPods(podList)
}

// A minimal kubelet client. It assumes the kubelet can be reached the kubelet's insecure API at
// 127.0.0.1:10255 and the secure API at 127.0.0.1:10250.
type kubeletClient struct {
	insecureClient *rest.RESTClient
	secureClient   *rest.RESTClient
	criClient      *RemoteRuntimeService
}

func newKubeletClient(config *rest.Config) (*kubeletClient, error) {
	// Use the core API group serializer. Same logic as client-go.
	// https://github.com/kubernetes/client-go/blob/v3.0.0/kubernetes/typed/core/v1/core_client.go#L147
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	// Kubelet is using a self-signed cert.
	config.TLSClientConfig.Insecure = true
	config.TLSClientConfig.CAFile = ""
	config.TLSClientConfig.CAData = nil

	// Shallow copy.
	insecureConfig := *config
	secureConfig := *config

	insecureConfig.Host = "http://127.0.0.1:10255"
	secureConfig.Host = "https://127.0.0.1:10250"

	client := new(kubeletClient)
	var err error
	if client.insecureClient, err = rest.UnversionedRESTClientFor(&insecureConfig); err != nil {
		return nil, fmt.Errorf("failed creating kubelet client for debug endpoints: %v", err)
	}
	if client.secureClient, err = rest.UnversionedRESTClientFor(&secureConfig); err != nil {
		return nil, fmt.Errorf("failed creating kubelet client: %v", err)
	}

	// Open a GRPC connection to the CRI shim
	client.criClient, err = NewRemoteRuntimeService(remoteRuntimeEndpoint, runtimeRequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CRI server: %v", err)
	}

	return client, nil
}

func NewRemoteRuntimeService(endpoint string, connectionTimeout time.Duration) (*RemoteRuntimeService, error) {
	glog.Infof("Connecting to runtime service %s", endpoint)
	addr, dialer, err := util.GetAddressAndDialer(endpoint)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(connectionTimeout), grpc.WithDialer(dialer))
	if err != nil {
		glog.Errorf("Connect remote runtime %s failed: %v", addr, err)
		return nil, err
	}

	return &RemoteRuntimeService{
		timeout:       connectionTimeout,
		runtimeClient: runtimeapi.NewRuntimeServiceClient(conn),
	}, nil
}

// localParentPods will retrieve all pods from kubelet api that are parents & should be checkpointed
func (c *kubeletClient) localParentPods() map[string]*v1.Pod {
	podList := new(v1.PodList)
	if err := c.insecureClient.Get().AbsPath("/pods/").Do().Into(podList); err != nil {
		// Assume there are no local parent pods.
		glog.Errorf("failed to list local parent pods, assuming none are running: %v", err)
	}
	return podListToParentPods(podList)
}

// localRunningPods uses the CRI shim to retrieve the local container runtime pod state
func (c *kubeletClient) localRunningPods() map[string]*v1.Pod {

	pods := make(map[string]*v1.Pod)

	// Retrieving sandboxes is likely redudant but is done to maintain sameness with what the kubelet does
	sandboxes, err := c.getRunningKubeletSandboxes()
	if err != nil {
		glog.Errorf("failed to list running sandboxes: %v", err)
		return nil
	}

	// Add pods from all sandboxes
	for _, s := range sandboxes {
		if s.Metadata == nil {
			glog.V(4).Infof("Sandbox does not have metadata: %+v", s)
			continue
		}

		podName := s.Metadata.Namespace + "/" + s.Metadata.Name
		if _, ok := pods[podName]; !ok {
			p := &v1.Pod{}
			p.UID = types.UID(s.Metadata.Uid)
			p.Name = s.Metadata.Name
			p.Namespace = s.Metadata.Namespace

			pods[podName] = p
		}
	}

	containers, err := c.getRunningKubeletContainers()
	if err != nil {
		glog.Errorf("failed to list running containers: %v", err)
		return nil
	}

	// Add all pods that containers are apart of
	for _, c := range containers {
		if c.Metadata == nil {
			glog.V(4).Infof("Container does not have metadata: %+v", c)
			continue
		}

		podName := c.Labels[kubelettypes.KubernetesPodNamespaceLabel] + "/" + c.Labels[kubelettypes.KubernetesPodNameLabel]
		if _, ok := pods[podName]; !ok {
			p := &v1.Pod{}
			p.UID = types.UID(c.Labels[kubelettypes.KubernetesPodUIDLabel])
			p.Name = c.Labels[kubelettypes.KubernetesPodNameLabel]
			p.Namespace = c.Labels[kubelettypes.KubernetesPodNamespaceLabel]

			pods[podName] = p
		}
	}

	return pods
}

func (c *kubeletClient) getRunningKubeletContainers() ([]*runtimeapi.Container, error) {
	filter := &runtimeapi.ContainerFilter{}

	// Filter out non-running containers
	filter.State = &runtimeapi.ContainerStateValue{
		State: runtimeapi.ContainerState_CONTAINER_RUNNING,
	}

	return c.criClient.listContainers(filter)
}

func (c *kubeletClient) getRunningKubeletSandboxes() ([]*runtimeapi.PodSandbox, error) {
	filter := &runtimeapi.PodSandboxFilter{}

	// Filter out non-running sandboxes
	filter.State = &runtimeapi.PodSandboxStateValue{
		State: runtimeapi.PodSandboxState_SANDBOX_READY,
	}
	return c.criClient.listPodSandbox(filter)
}

// checkpointSecretVolumes ensures that all pod secrets are checkpointed locally, then converts the secret volume to a hostpath.
func checkpointSecretVolumes(client kubernetes.Interface, pod *v1.Pod) (*v1.Pod, error) {
	for i := range pod.Spec.Volumes {
		v := &pod.Spec.Volumes[i]
		if v.Secret == nil {
			continue
		}

		_, err := checkpointSecret(client, pod.Namespace, pod.Name, v.Secret.SecretName)
		if err != nil {
			return nil, fmt.Errorf("failed to checkpoint secret for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
	return pod, nil
}

// checkpointSecret will locally store secret data.
// The path to the secret data becomes: checkpointSecretPath/namespace/podname/secretName/secret.file
// Where each "secret.file" is a key from the secret.Data field.
func checkpointSecret(client kubernetes.Interface, namespace, podName, secretName string) (string, error) {
	secret, err := client.Core().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve secret %s/%s: %v", namespace, secretName, err)
	}

	basePath := secretPath(namespace, podName, secretName)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return "", fmt.Errorf("failed to create secret checkpoint path %s: %v", basePath, err)
	}
	// TODO(aaron): No need to store if already exists
	for f, d := range secret.Data {
		if err := writeAndAtomicRename(filepath.Join(basePath, f), d, 0600); err != nil {
			return "", fmt.Errorf("failed to write secret %s: %v", secret.Name, err)
		}
	}
	return basePath, nil
}

// checkpointConfigMapVolumes ensures that all pod configMaps are checkpointed locally, then converts the configMap volume to a hostpath.
func checkpointConfigMapVolumes(client kubernetes.Interface, pod *v1.Pod) (*v1.Pod, error) {
	for i := range pod.Spec.Volumes {
		v := &pod.Spec.Volumes[i]
		if v.ConfigMap == nil {
			continue
		}

		_, err := checkpointConfigMap(client, pod.Namespace, pod.Name, v.ConfigMap.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to checkpoint configMap for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
	return pod, nil
}

// checkpointConfigMap will locally store configMap data.
// The path to the configMap data becomes: checkpointSecretPath/namespace/podname/configMapName/configMap.file
// Where each "configMap.file" is a key from the configMap.Data field.
func checkpointConfigMap(client kubernetes.Interface, namespace, podName, configMapName string) (string, error) {
	configMap, err := client.Core().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve configMap %s/%s: %v", namespace, configMapName, err)
	}

	basePath := configMapPath(namespace, podName, configMapName)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return "", fmt.Errorf("failed to create configMap checkpoint path %s: %v", basePath, err)
	}
	// TODO(aaron): No need to store if already exists
	for f, d := range configMap.Data {
		if err := writeAndAtomicRename(filepath.Join(basePath, f), []byte(d), 0600); err != nil {
			return "", fmt.Errorf("failed to write configMap %s: %v", configMap.Name, err)
		}
	}
	return basePath, nil
}

func handleRemove(remove []string) {
	for _, id := range remove {
		glog.Infof("Removing checkpoint of: %s", id)

		// Remove Secrets
		p := PodFullNameToSecretPath(id)
		if err := os.RemoveAll(p); err != nil {
			glog.Errorf("Failed to remove pod secrets from %s: %s", p, err)
		}

		// Remove ConfipMaps
		p = PodFullNameToConfigMapPath(id)
		if err := os.RemoveAll(p); err != nil {
			glog.Errorf("Failed to remove pod configMaps from %s: %s", p, err)
		}

		// Remove inactive checkpoints
		p = PodFullNameToInactiveCheckpointPath(id)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			glog.Errorf("Failed to remove inactive checkpoint %s: %v", p, err)
			continue
		}

		// Remove active checkpoints.
		// We do this as the last step because we want to clean up
		// resources before the checkpointer itself exits.
		//
		// TODO(yifan): Removing the pods after removing the secrets/configmaps
		// might disturb other pods since they might want to use the configmap
		// or secrets during termination.
		//
		// However, since we are not waiting for them to terminate anyway, so it's
		// ok to just leave as is for now. We can handle this more gracefully later.
		p = PodFullNameToActiveCheckpointPath(id)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			glog.Errorf("Failed to remove active checkpoint %s: %v", p, err)
			continue
		}
	}
}

func handleStop(stop []string) {
	for _, id := range stop {
		glog.Infof("Stopping active checkpoint: %s", id)
		p := PodFullNameToActiveCheckpointPath(id)
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) { // Sanity check (it's fine - just want to surface this if it's occurring)
				glog.Warningf("Attempted to remove active checkpoint, but manifest no longer exists: %s", p)
			} else {
				glog.Errorf("Failed to stop active checkpoint %s: %v", p, err)
			}
		}
	}
}

func handleStart(start []string) {
	for _, id := range start {
		src := PodFullNameToInactiveCheckpointPath(id)
		data, err := ioutil.ReadFile(src)
		if err != nil {
			glog.Errorf("Failed to read checkpoint source: %v", err)
			continue
		}

		dst := PodFullNameToActiveCheckpointPath(id)
		if _, err := writeManifestIfDifferent(dst, id, data); err != nil {
			glog.Errorf("Failed to write active checkpoint manifest: %v", err)
		}
	}
}

func podListToParentPods(pl *v1.PodList) map[string]*v1.Pod {
	return podListToMap(pl, isValidParent)
}

func filterNone(p *v1.Pod) bool {
	return true
}

type filterFn func(*v1.Pod) bool

func podListToMap(pl *v1.PodList, filter filterFn) map[string]*v1.Pod {
	pods := make(map[string]*v1.Pod)
	for i := range pl.Items {
		if !filter(&pl.Items[i]) {
			continue
		}

		pod := &pl.Items[i]
		id := PodFullName(pod)

		if _, ok := pods[id]; ok { // TODO(aaron): likely not be necessary (shouldn't ever happen) - but sanity check
			glog.Warningf("Found multiple local parent pods with same id: %s", id)
		}

		// Pods from Kubelet API do not have TypeMeta populated - set it here either way.
		pods[id] = pod
		pods[id].TypeMeta = metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		}
	}
	return pods
}

// A valid checkpoint parent:
//    has the checkpoint=true annotation
//    is not a static pod itself
//    is not a checkpoint pod itself
func isValidParent(pod *v1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}
	shouldCheckpoint := pod.Annotations[shouldCheckpointAnnotation] == shouldCheckpoint
	isStatic := pod.Annotations[podSourceAnnotation] == podSourceFile
	return shouldCheckpoint && !isStatic && !isCheckpoint(pod)
}

func isCheckpoint(pod *v1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}
	_, ok := pod.Annotations[checkpointParentAnnotation]
	return ok
}

func copyPod(pod *v1.Pod) (*v1.Pod, error) {
	obj, err := api.Scheme.Copy(pod)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Pod), nil
}

func PodFullName(pod *v1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}

func PodFullNameToInactiveCheckpointPath(id string) string {
	return filepath.Join(inactiveCheckpointPath, strings.Replace(id, "/", "-", -1)+".json")
}

func PodFullNameToActiveCheckpointPath(id string) string {
	return filepath.Join(activeCheckpointPath, strings.Replace(id, "/", "-", -1)+".json")
}

func secretPath(namespace, podName, secretName string) string {
	return filepath.Join(checkpointSecretPath, namespace, podName, secretName)
}

func PodFullNameToSecretPath(id string) string {
	namespace, podname := path.Split(id)
	return filepath.Join(checkpointSecretPath, namespace, podname)
}

func configMapPath(namespace, podName, configMapName string) string {
	return filepath.Join(checkpointConfigMapPath, namespace, podName, configMapName)
}

func PodFullNameToConfigMapPath(id string) string {
	namespace, podname := path.Split(id)
	return filepath.Join(checkpointConfigMapPath, namespace, podname)
}

func writeAndAtomicRename(path string, data []byte, perm os.FileMode) error {
	tmpfile := filepath.Join(filepath.Dir(path), "."+filepath.Base(path))
	if err := ioutil.WriteFile(tmpfile, data, perm); err != nil {
		return err
	}
	return os.Rename(tmpfile, path)
}

type RemoteRuntimeService struct {
	timeout       time.Duration
	runtimeClient runtimeapi.RuntimeServiceClient
}

func (r *RemoteRuntimeService) listPodSandbox(filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	resp, err := r.runtimeClient.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{
		Filter: filter,
	})
	if err != nil {
		glog.Errorf("ListPodSandbox with filter %q from runtime sevice failed: %v", filter, err)
		return nil, err
	}

	return resp.Items, nil
}

func (r *RemoteRuntimeService) listContainers(filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	resp, err := r.runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: filter,
	})
	if err != nil {
		glog.Errorf("ListContainers with filter %q from runtime service failed: %v", filter, err)
		return nil, err
	}

	return resp.Containers, nil
}

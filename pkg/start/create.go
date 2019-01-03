package start

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	crdRolloutDuration = 1 * time.Second
	crdRolloutTimeout  = 2 * time.Minute
)

func CreateAssets(config clientcmd.ClientConfig, manifestDir string, timeout time.Duration) error {
	if _, err := os.Stat(manifestDir); os.IsNotExist(err) {
		UserOutput(fmt.Sprintf("WARNING: %v does not exist, not creating any self-hosted assets.\n", manifestDir))
		return nil
	}
	clientConfig, err := config.ClientConfig()
	if err != nil {
		return err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	creater := newCreater(dynamicClient, discoveryClient)

	manifests, err := creater.loadManifests(manifestDir)
	if err != nil {
		return fmt.Errorf("loading manifests: %v", err)
	}

	upFn := func() (bool, error) {
		if err := apiTest(config); err != nil {
			glog.Warningf("Unable to determine api-server readiness: %v", err)
			return false, nil
		}
		return true, nil
	}

	UserOutput("Waiting for api-server...\n")
	if err := wait.Poll(5*time.Second, timeout, upFn); err != nil {
		err = fmt.Errorf("API Server is not ready: %v", err)
		glog.Error(err)
		return err
	}

	UserOutput("Creating self-hosted assets...\n")
	if err := creater.createManifests(manifests); err != nil {
		UserOutput("\nNOTE: Bootkube failed to create some cluster assets. It is important that manifest errors are resolved and resubmitted to the apiserver.\n")
		UserOutput("For example, after resolving issues: kubectl create -f <failed-manifest>\n\n")
		UserOutput("Errors:\n%v\n\n", err)
	}

	return nil
}

func apiTest(c clientcmd.ClientConfig) error {
	config, err := c.ClientConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// API Server is responding
	healthStatus := 0
	client.Discovery().RESTClient().Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
	if healthStatus != http.StatusOK {
		return fmt.Errorf("API Server http status: %d", healthStatus)
	}

	// System namespace has been created
	_, err = client.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
	return err
}

type errorReporter struct {
	errors []error
}

func (r *errorReporter) errorf(format string, obj ...interface{}) {
	err := fmt.Errorf(format, obj...)
	UserOutput("ERROR: %v\n", err)
	r.errors = append(r.errors, err)
}

func (r *errorReporter) warningf(format string, obj ...interface{}) {
	UserOutput("WARNING: %v\n", fmt.Errorf("%v", obj...))
}

func (r *errorReporter) allErrors() error {
	if !r.hasErrors() {
		return nil
	}
	var msgs []string
	for _, err := range r.errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("%s", strings.Join(msgs, "\n"))
}

func (r *errorReporter) hasErrors() bool {
	return len(r.errors) > 0
}

type creater struct {
	client dynamic.Interface
	// mapper maps resource kinds ("ConfigMap") with their pluralized URL
	// path ("configmaps") using the discovery APIs.
	mapper *resourceMapper
}

func newCreater(client dynamic.Interface, discoveryClient *discovery.DiscoveryClient) *creater {
	return &creater{
		client: client,
		mapper: newResourceMapper(discoveryClient),
	}
}

func (c *creater) createManifests(manifests map[string]*unstructured.Unstructured) error {
	// Bootkube used to create manifests in named order ("01-foo" before "02-foo").
	// Maintain this behavior for everything except CRDs and NSs, which have strict ordering
	// that we should always respect.
	var sortedManifests []string
	for key := range manifests {
		sortedManifests = append(sortedManifests, key)
	}
	sort.Strings(sortedManifests)

	var (
		namespaces []*unstructured.Unstructured
		crds       []*unstructured.Unstructured
		others     []*unstructured.Unstructured
	)

	for _, path := range sortedManifests {
		// fmt.Printf("got: %+v\n", c.mapper.groupVersionResource(manifests[path]))
		switch c.mapper.groupVersionResource(manifests[path]) {
		case schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}:
			namespaces = append(namespaces, manifests[path])
		case schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1beta1", Resource: "customresourcedefinitions"}:
			crds = append(crds, manifests[path])
		default:
			others = append(others, manifests[path])
		}
	}

	report := errorReporter{}

	// Create all namespaces first
	for _, namespace := range namespaces {
		UserOutput("Creating %s %q ...\n", namespace.GroupVersionKind(), namespace.GetName())
		if _, err := c.client.Resource(c.mapper.groupVersionResource(namespace)).Create(namespace); err != nil {
			if errors.IsAlreadyExists(err) {
				report.warningf("%s: %v", err)
				continue
			}
			report.errorf("Failed to create %s %q: %v", namespace.GroupVersionKind(), namespace.GetName(), err)
			return err
		}
	}

	// We can't continue when namespaces failed to created
	if report.hasErrors() {
		return report.allErrors()
	}

	// Create the custom resource definition before creating the actual custom resources.
	for _, crd := range crds {
		UserOutput("Creating %s %q ...\n", crd.GroupVersionKind(), crd.GetName())
		if _, err := c.client.Resource(c.mapper.groupVersionResource(crd)).Create(crd); err != nil {
			if errors.IsAlreadyExists(err) {
				report.warningf("%s: %v", crd, err)
				continue
			}
			report.errorf("Failed to create %s %q: %v", crd.GroupVersionKind(), crd.GetName(), err)
			continue
		}
		crdGroup, _, err := unstructured.NestedString(crd.UnstructuredContent(), "spec", "group")
		if err != nil {
			report.errorf("Expected spec.group field in %s: %v", crd, err)
			continue
		}
		crdPlural, _, err := unstructured.NestedString(crd.UnstructuredContent(), "spec", "names", "plural")
		if err != nil {
			report.errorf("Expected spec.names.plural field in %s: %v", crd, err)
			continue
		}

		crdVersion := ""
		crdVersions, _, err := unstructured.NestedSlice(crd.UnstructuredContent(), "spec", "versions")
		if err != nil {
			report.errorf("Expected spec.versions field in %s: %v", crd, err)
			continue
		}

		if len(crdVersions) > 0 {
			for i, v := range crdVersions {
				version := v.(map[string]interface{})
				if isServed, _, _ := unstructured.NestedBool(version, "served"); isServed {
					versionName, _, err := unstructured.NestedString(version, "name")
					if err != nil {
						report.errorf("Expected spec.versions[%d].name field in %s: %v", i, crd, err)
						continue
					}
					crdVersion = versionName
				}
			}
		} else {
			// TODO: This field is deprecated.
			crdSpecVersion, _, err := unstructured.NestedString(crd.UnstructuredContent(), "spec", "version")
			if err != nil {
				report.errorf("Expected spec.version field in %s: %v", crd, err)
				continue
			}
			crdVersion = crdSpecVersion
		}

		crGVR := schema.GroupVersionResource{Group: crdGroup, Version: crdVersion, Resource: crdPlural}

		// Wait for the CRD to be available
		UserOutput("Waiting for %s %q to be available ...\n", crGVR.GroupVersion(), crGVR.Resource)
		waitErr := wait.PollImmediate(crdRolloutDuration, crdRolloutTimeout, func() (bool, error) {
			_, err := c.client.Resource(crGVR).List(metav1.ListOptions{})
			if errors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return true, nil
		})
		if waitErr != nil {
			report.errorf("Failed to wait for %q to be available: %v", crGVR.Resource, waitErr)
		}
	}

	// If some CRDs failed to create, do not continue
	if report.hasErrors() {
		return report.allErrors()
	}

	// Create other resources
	for _, resource := range others {
		UserOutput("Creating %s %q ...\n", resource.GroupVersionKind(), resource.GetName())
		if c.mapper.isNamespaced(resource) {
			if _, err := c.client.Resource(c.mapper.groupVersionResource(resource)).Namespace(resource.GetNamespace()).Create(resource); err != nil {
				if errors.IsAlreadyExists(err) {
					report.warningf("%s: %v", err)
					continue
				}
				report.errorf("Failed to create %s %q: %v", resource.GroupVersionKind(), resource.GetName(), err)
			}
			continue
		}
		if _, err := c.client.Resource(c.mapper.groupVersionResource(resource)).Create(resource); err != nil {
			if errors.IsAlreadyExists(err) {
				report.warningf("%s: %v", err)
				continue
			}
			report.errorf("Failed to create %s %q: %v", resource.GroupVersionKind(), resource.GetName(), err)
		}
	}

	return report.allErrors()
}

// loadManifests parses a directory of YAML Kubernetes manifest.
func (c *creater) loadManifests(p string) (map[string]*unstructured.Unstructured, error) {
	manifests := map[string]*unstructured.Unstructured{}
	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			b := filepath.Base(p)
			if b != p && strings.HasPrefix(b, ".") {
				// Ignore directories that start with a "."
				return filepath.SkipDir
			}
			return nil
		}

		manifestBytes, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		manifestJSON, err := yaml.YAMLToJSON(manifestBytes)
		if err != nil {
			return err
		}
		manifestObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, manifestJSON)
		if err != nil {
			return err
		}
		manifestObject, ok := manifestObj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unable to convert %+v to unstructed", manifestObj)
		}

		manifests[path] = manifestObject
		return nil
	})
	return manifests, err
}

func newResourceMapper(d discovery.DiscoveryInterface) *resourceMapper {
	return &resourceMapper{d, sync.Mutex{}, make(map[string]*metav1.APIResourceList)}
}

// resourceMapper uses the Kubernetes discovery APIs to map a resource Kind to its pluralized
// name to construct a URL path. For example, "ClusterRole" would be converted to "clusterroles".
//
// NOTE(ericchiang): I couldn't get discovery.DeferredDiscoveryRESTMapper working for the
// life of me. This implements the same logic.
type resourceMapper struct {
	discoveryClient discovery.DiscoveryInterface

	mu    sync.Mutex
	cache map[string]*metav1.APIResourceList
}

// resourceInfo uses the API server discovery APIs to determine the resource definition
// of a given Kind.
func (m *resourceMapper) resourceInfo(groupVersion, kind string) (*metav1.APIResource, error) {
	m.mu.Lock()
	l, ok := m.cache[groupVersion]
	m.mu.Unlock()

	if ok {
		// Check cache.
		for _, r := range l.APIResources {
			if r.Kind == kind {
				return &r, nil
			}
		}
	}

	l, err := m.discoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return nil, fmt.Errorf("discover group version %s: %v", groupVersion, err)
	}

	m.mu.Lock()
	m.cache[groupVersion] = l
	m.mu.Unlock()

	for _, r := range l.APIResources {
		if r.Kind == kind {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("resource %s %s not found", groupVersion, kind)
}

func (m *resourceMapper) isNamespaced(obj *unstructured.Unstructured) bool {
	apiResource, err := m.resourceInfo(obj.GetAPIVersion(), obj.GetKind())
	if err != nil {
		panic(err)
	}
	return apiResource.Namespaced
}

func (m *resourceMapper) groupVersionResource(obj *unstructured.Unstructured) schema.GroupVersionResource {
	apiResource, err := m.resourceInfo(obj.GetAPIVersion(), obj.GetKind())
	if err != nil {
		panic(err)
	}
	return schema.GroupVersionResource{
		Group:    apiResource.Group,
		Version:  apiResource.Version,
		Resource: apiResource.Name,
	}
}

func (m *resourceMapper) toString(resource *unstructured.Unstructured) string {
	if m.isNamespaced(resource) {
		return fmt.Sprintf("%s %s", resource.GetKind(), resource.GetName())
	}
	return fmt.Sprintf("%s %s/%s", resource.GetKind(), resource.GetNamespace(), resource.GetName())
}

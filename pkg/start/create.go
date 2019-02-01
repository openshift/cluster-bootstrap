package start

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	crdRolloutDuration = 1 * time.Second
	crdRolloutTimeout  = 2 * time.Minute
)

func createAssets(c *rest.Config, manifestDir string, timeout time.Duration, strict bool, retryDelay time.Duration) error {
	if _, err := os.Stat(manifestDir); os.IsNotExist(err) {
		UserOutput(fmt.Sprintf("WARNING: %v does not exist, not creating any self-hosted assets.\n", manifestDir))
		return nil
	}
	creater, err := newCreater(c, strict)
	if err != nil {
		return err
	}

	m, err := loadManifests(manifestDir)
	if err != nil {
		return fmt.Errorf("loading manifests: %v", err)
	}

	upFn := func() (bool, error) {
		if err := apiTest(c); err != nil {
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
	if ok := creater.createManifests(m, retryDelay); !ok {
		UserOutput("\nNOTE: Bootkube failed to create some cluster assets. It is important that manifest errors are resolved and resubmitted to the apiserver.\n")
		UserOutput("For example, after resolving issues: kubectl create -f <failed-manifest>\n\n")

		// Don't fail on manifest creation. It's easier to debug a cluster with a failed
		// manifest than exiting and tearing down the control plane. If strict
		// mode is enabled, then error out.
		if strict {
			return fmt.Errorf("Self-hosted assets could not be created")
		}
	}

	return nil
}

func apiTest(config *rest.Config) error {
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

type manifest struct {
	kind       string
	apiVersion string
	namespace  string
	name       string
	raw        []byte

	filepath string
}

func (m manifest) String() string {
	if m.namespace == "" {
		return fmt.Sprintf("%s %s %s", m.filepath, m.kind, m.name)
	}
	return fmt.Sprintf("%s %s %s/%s", m.filepath, m.kind, m.namespace, m.name)
}

type creater struct {
	client *rest.RESTClient
	strict bool

	// mapper maps resource kinds ("ConfigMap") with their pluralized URL
	// path ("configmaps") using the discovery APIs.
	mapper *resourceMapper
}

func newCreater(c *rest.Config, strict bool) (*creater, error) {
	c.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	client, err := rest.UnversionedRESTClientFor(c)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		return nil, err
	}

	return &creater{
		mapper: newResourceMapper(discoveryClient),
		client: client,
		strict: strict,
	}, nil
}

func (c *creater) createManifests(manifests []manifest, retryDelay time.Duration) (ok bool) {
	ok = true
	// Bootkube used to create manifests in named order ("01-foo" before "02-foo").
	// Maintain this behavior for everything except CRDs and NSs, which have strict ordering
	// that we should always respect.
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].filepath < manifests[j].filepath
	})

	var namespaces, crds, other []manifest
	for _, m := range manifests {
		if m.kind == "CustomResourceDefinition" && strings.HasPrefix(m.apiVersion, "apiextensions.k8s.io/") {
			crds = append(crds, m)
		} else if m.kind == "Namespace" && m.apiVersion == "v1" {
			namespaces = append(namespaces, m)
		} else {
			other = append(other, m)
		}
	}

	create := func(m manifest) error {
		if err := c.create(m, retryDelay); err != nil {
			ok = false
			UserOutput("Failed creating %s: %v\n", m, err)
			return err
		}
		UserOutput("Created %s\n", m)
		return nil
	}

	// Create all namespaces first
	for _, m := range namespaces {
		if err := create(m); err != nil && c.strict {
			return false
		}
	}

	// Create the custom resource definition before creating the actual custom resources.
	for _, m := range crds {
		if err := create(m); err != nil && c.strict {
			return false
		}
	}

	// Wait until the API server registers the CRDs. Until then it's not safe to create the
	// manifests for those custom resources.
	for _, crd := range crds {
		if err := c.waitForCRD(crd); err != nil {
			ok = false
			UserOutput("Failed waiting for %s: %v", crd, err)
			if c.strict {
				return false
			}
		}
	}

	for _, m := range other {
		if err := create(m); err != nil && c.strict {
			return false
		}
	}
	return ok
}

// waitForCRD blocks until the API server begins serving the custom resource this
// manifest defines. This is determined by listing the custom resource in a loop.
func (c *creater) waitForCRD(m manifest) error {
	var crd apiextensionsv1beta1.CustomResourceDefinition
	if err := json.Unmarshal(m.raw, &crd); err != nil {
		return fmt.Errorf("failed to unmarshal manifest: %v", err)
	}

	// get first served version
	firstVer := ""
	if len(crd.Spec.Versions) > 0 {
		for _, v := range crd.Spec.Versions {
			if v.Served {
				firstVer = v.Name
				break
			}
		}
	} else {
		firstVer = crd.Spec.Version
	}
	if len(firstVer) == 0 {
		return fmt.Errorf("expected at least one served version")
	}

	return wait.PollImmediate(crdRolloutDuration, crdRolloutTimeout, func() (bool, error) {
		// get all resources, giving a 200 result with empty list on success, 404 before the CRD is active.
		namespaceLessURI := allCustomResourcesURI(schema.GroupVersionResource{Group: crd.Spec.Group, Version: firstVer, Resource: crd.Spec.Names.Plural})
		res := c.client.Get().RequestURI(namespaceLessURI).Do()
		if res.Error() != nil {
			if errors.IsNotFound(res.Error()) {
				return false, nil
			}
			return false, res.Error()
		}
		return true, nil
	})
}

// allCustomResourcesURI returns the URI for the CRD resource without a namespace, listing
// all objects of that GroupVersionResource.
func allCustomResourcesURI(gvr schema.GroupVersionResource) string {
	return fmt.Sprintf("/apis/%s/%s/%s",
		strings.ToLower(gvr.Group),
		strings.ToLower(gvr.Version),
		strings.ToLower(gvr.Resource),
	)
}

func (c *creater) create(m manifest, retryDelay time.Duration) error {
	info, err := c.mapper.resourceInfo(m.apiVersion, m.kind)
	if err != nil {
		return fmt.Errorf("dicovery failed: %v", err)
	}

	for {
		err = c.client.Post().
			AbsPath(m.urlPath(info.Name, info.Namespaced)).
			Body(m.raw).
			SetHeader("Content-Type", "application/json").
			Do().Error()
		if err == nil || retryDelay == 0 {
			return nil
		}
		UserOutput(fmt.Sprintf("Failed creating %s: %v (will retry in %v)\n", m, err, retryDelay))
	}
}

func (m manifest) urlPath(plural string, namespaced bool) string {
	u := "/apis"
	if m.apiVersion == "v1" {
		u = "/api"
	}
	u = u + "/" + m.apiVersion
	// NOTE(ericchiang): Some of our non-namespaced manifests have a "namespace" field.
	// Since kubectl create accepts this, also accept this.
	if m.namespace != "" && namespaced {
		u = u + "/namespaces/" + m.namespace
	}
	return u + "/" + plural
}

// loadManifests parses a directory of YAML Kubernetes manifest.
func loadManifests(p string) ([]manifest, error) {
	var manifests []manifest
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

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %v", path, err)
		}
		defer f.Close()

		ms, err := parseManifests(f)
		if err != nil {
			return fmt.Errorf("parse file %s: %v", path, err)
		}
		for i := range ms {
			ms[i].filepath = path
		}
		manifests = append(manifests, ms...)
		return nil
	})
	return manifests, err
}

// parseManifests parses a YAML or JSON document that may contain one or more
// kubernetes resoures.
func parseManifests(r io.Reader) ([]manifest, error) {
	reader := yaml.NewYAMLReader(bufio.NewReader(r))
	var manifests []manifest
	for {
		yamlManifest, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				if len(manifests) == 0 {
					return nil, fmt.Errorf("no resources found")
				}
				return manifests, nil
			}
			return nil, err
		}
		yamlManifest = bytes.TrimSpace(yamlManifest)
		if len(yamlManifest) == 0 {
			continue
		}

		jsonManifest, err := yaml.ToJSON(yamlManifest)
		if err != nil {
			return nil, fmt.Errorf("invalid manifest: %v", err)
		}
		m, err := parseJSONManifest(jsonManifest)
		if err != nil {
			return nil, fmt.Errorf("parse manifest: %v", err)
		}
		manifests = append(manifests, m)
	}
}

// parseJSONManifest parses a single JSON Kubernetes resource.
func parseJSONManifest(data []byte) (manifest, error) {
	var m struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("parse manifest: %v", err)
	}
	return manifest{
		kind:       m.Kind,
		apiVersion: m.APIVersion,
		namespace:  m.Metadata.Namespace,
		name:       m.Metadata.Name,
		raw:        data,
	}, nil
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

package start

import (
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestCreateLoadManifests(t *testing.T) {
	c := &creater{}
	result, err := c.loadManifests("testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 12 {
		t.Fatalf("expected 12 loaded manifests, got %d", len(result))
	}
}

func TestCreateManifests(t *testing.T) {
	fakeScheme := runtime.NewScheme()

	// TODO: This is a workaround for dynamic fake client bug where the List kind is enforced and duplicated in object reactor.
	fakeScheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ListList"}, &unstructured.UnstructuredList{})

	dynamicClient := dynamicfake.NewSimpleDynamicClient(fakeScheme)
	fakeDiscovery := &fake.FakeDiscovery{
		Fake:               &dynamicClient.Fake,
		FakedServerVersion: &version.Info{},
	}

	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "kubeapiserver.operator.openshift.io/v1alpha1",
			APIResources: []metav1.APIResource{
				{
					Kind:    "KubeAPIServerOperatorConfig",
					Name:    "kubeapiserveroperatorconfigs",
					Version: "v1alpha1",
					Group:   "kubeapiserver.operator.openshift.io",
				},
			},
		},
		{
			GroupVersion: "apiextensions.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{
					Kind:    "CustomResourceDefinition",
					Name:    "customresourcedefinitions",
					Version: "v1beta1",
					Group:   "apiextensions.k8s.io",
				},
			},
		},
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Version: "v1",
					Kind:    "Namespace",
					Name:    "namespaces",
				},
				{
					Version:    "v1",
					Kind:       "ConfigMap",
					Name:       "configmaps",
					Namespaced: true,
				},
				{
					Version:    "v1",
					Kind:       "Secret",
					Name:       "secrets",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Version: "v1",
					Group:   "rbac.authorization.k8s.io",
					Name:    "clusterrolebindings",
					Kind:    "ClusterRoleBinding",
				},
				{
					Version: "v1",
					Group:   "rbac.authorization.k8s.io",
					Name:    "clusterroles",
					Kind:    "ClusterRole",
				},
			},
		},
	}

	c := newCreater(dynamicClient, nil)
	c.mapper = &resourceMapper{
		discoveryClient: fakeDiscovery,
		mu:              sync.Mutex{},
		cache:           make(map[string]*metav1.APIResourceList),
	}

	result, _ := c.loadManifests("testdata")
	if len(result) != 12 {
		t.Fatalf("expected 12 loaded manifests, got %d", len(result))
	}
	if err := c.createManifests(result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	createActions := []clienttesting.CreateAction{}
	for _, action := range dynamicClient.Actions() {
		if createAction, ok := action.(clienttesting.CreateAction); ok {
			createActions = append(createActions, createAction)
		}
	}
	if len(createActions) != 12 {
		t.Fatalf("expected 12 create actions, got %d", len(createActions))
	}

	if createActions[0].GetResource().Resource != "namespaces" {
		t.Fatalf("expected to create namespace first, got %+v", createActions[0])
	}
}

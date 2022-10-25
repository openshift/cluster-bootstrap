package start

import (
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const (
	assetPathSecrets            = "tls"
	assetPathAdminKubeConfig    = "auth/kubeconfig-loopback"
	assetPathClusterConfig      = "manifests/cluster-config.yaml"
	assetPathManifests          = "manifests"
	assetPathBootstrapManifests = "bootstrap-manifests"
)

var (
	bootstrapSecretsDir = "/etc/kubernetes/bootstrap-secrets" // Overridden for testing.
)

func getUnstructured(file string) (*unstructured.Unstructured, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	json, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}
	obj, err := apiruntime.Decode(unstructured.UnstructuredJSONScheme, json)
	if err != nil {
		return nil, err
	}
	config, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unexpected object in %t", obj)
	}
	return config, nil
}

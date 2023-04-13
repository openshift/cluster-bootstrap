package start

import (
	"fmt"
	"github.com/openshift/installer/pkg/types"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
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

func getInstallConfig(file string) (*types.InstallConfig, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	cm := v1.ConfigMap{}
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster config cm %w", err)
	}
	installConfigData, ok := cm.Data["install-config"]
	if !ok {
		return nil, fmt.Errorf("install-config doesn't exist in cluster config cm %w", err)
	}

	installConfig := types.InstallConfig{}
	if err := yaml.Unmarshal([]byte(installConfigData), &installConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal install config %w", err)
	}

	return &installConfig, nil
}

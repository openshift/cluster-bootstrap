package start

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/openshift/installer/pkg/types"
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

// bubbleUpCRDs ensures that openshift/api CRDs are applied before other types as kubeapiserver
// readyz depends on them
func bubbleUpCRDs(manifestsPath string) error {
	return filepath.WalkDir(manifestsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".crd.yaml") {
			// Split by underscore and set runlevel to `0000` so that it would run before CVO's runlevel 00
			fileName := filepath.Base(path)
			fileNameParts := strings.Split(fileName, "_")
			if len(fileNameParts) < 3 {
				UserOutput("Unable to determine runtime for filename %s\n", fileName)
				return nil
			}
			newFileNameParts := []string{fileNameParts[0], "0000"}
			newFileNameParts = append(newFileNameParts, fileNameParts[2:]...)
			newFileName := strings.Join(newFileNameParts, "_")
			UserOutput("Copying %s to %s to have CRDs applied first\n", fileName, newFileName)

			return copyFile(filepath.Join(manifestsPath, fileName), filepath.Join(manifestsPath, newFileName), true /* overwrite */)
		}
		return nil
	})
}

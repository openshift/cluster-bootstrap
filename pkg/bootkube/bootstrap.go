package bootkube

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
)

// CreateBootstrapControlPlane seeds static manifests to the kubelet to launch the bootstrap control
// plane.
func CreateBootstrapControlPlane(assetDir string, podManifestPath string) error {
	UserOutput("Running temporary bootstrap control plane...\n")

	// Make secrets temporarily available to bootstrap cluster.
	if err := os.RemoveAll(asset.BootstrapSecretsDir); err != nil {
		return err
	}
	if err := os.Mkdir(asset.BootstrapSecretsDir, os.FileMode(0700)); err != nil {
		return err
	}
	secretsDir := filepath.Join(assetDir, asset.AssetPathSecrets)
	secrets, err := ioutil.ReadDir(secretsDir)
	if err != nil {
		return err
	}
	for _, secret := range secrets {
		if err := copyFile(filepath.Join(secretsDir, secret.Name()), filepath.Join(asset.BootstrapSecretsDir, secret.Name()), true); err != nil {
			return err
		}
	}

	// Copy the static manifests to the kubelet's pod manifest path.
	manifestsDir := filepath.Join(assetDir, asset.AssetPathBootstrapManifests)
	manifests, err := ioutil.ReadDir(manifestsDir)
	if err != nil {
		return err
	}
	for _, manifest := range manifests {
		if err := copyFile(filepath.Join(manifestsDir, manifest.Name()), filepath.Join(podManifestPath, manifest.Name()), false); err != nil {
			return err
		}
	}

	return nil
}

// CleanupBootstrapControlPlane brings down the bootstrap control plane and cleans up the temporary
// secrets. This function is idempotent.
func CleanupBootstrapControlPlane(assetDir string, podManifestPath string) error {
	UserOutput("Cleaning up temporary bootstrap control plane...\n")

	if err := os.RemoveAll(asset.BootstrapSecretsDir); err != nil {
		return err
	}
	manifests, err := ioutil.ReadDir(filepath.Join(assetDir, asset.AssetPathBootstrapManifests))
	if err != nil {
		return err
	}
	for _, manifest := range manifests {
		if err := os.Remove(filepath.Join(podManifestPath, manifest.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, overwrite bool) error {
	if !overwrite {
		fi, err := os.Stat(dst)
		if fi != nil {
			return fmt.Errorf("file already exists: %v", dst)
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, os.FileMode(0600))
}

package checkpoint

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

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
			if _, ok := checkpoints[podFullName(cp)]; ok { // sanity check
				glog.Warningf("Found multiple checkpoint pods in %s with same id: %s", path, podFullName(cp))
			}
			checkpoints[podFullName(cp)] = cp
		}
	}
	return checkpoints
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
	return writeManifestIfDifferent(path, podFullName(pod), buff.Bytes())
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

func writeAndAtomicRename(path string, data []byte, perm os.FileMode) error {
	tmpfile := filepath.Join(filepath.Dir(path), "."+filepath.Base(path))
	if err := ioutil.WriteFile(tmpfile, data, perm); err != nil {
		return err
	}
	return os.Rename(tmpfile, path)
}

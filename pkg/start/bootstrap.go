package start

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type bootstrapControlPlane struct {
	assetDir        string
	podManifestPath string
	ownedManifests  []string
	teardownErrors  []error
}

// newBootstrapControlPlane constructs a new bootstrap control plane object.
func newBootstrapControlPlane(assetDir, podManifestPath string) *bootstrapControlPlane {
	return &bootstrapControlPlane{
		assetDir:        assetDir,
		podManifestPath: podManifestPath,
	}
}

// Start seeds static manifests to the kubelet to launch the bootstrap control plane.
// Users should always ensure that Cleanup() is called even in the case of errors.
func (b *bootstrapControlPlane) Start() error {
	UserOutput("Starting temporary bootstrap control plane...\n")
	// Make secrets temporarily available to bootstrap cluster.
	if err := os.RemoveAll(bootstrapSecretsDir); err != nil {
		return err
	}
	secretsDir := filepath.Join(b.assetDir, assetPathSecrets)
	if _, err := copyDirectory(secretsDir, bootstrapSecretsDir, true /* overwrite */); err != nil {
		return err
	}
	// Copy the admin kubeconfig. TODO(diegs): this is kind of a hack, maybe do something better.
	if err := copyFile(filepath.Join(b.assetDir, assetPathAdminKubeConfig), filepath.Join(bootstrapSecretsDir, "kubeconfig"), true /* overwrite */); err != nil {
		return err
	}

	// Copy the static manifests to the kubelet's pod manifest path.
	manifestsDir := filepath.Join(b.assetDir, assetPathBootstrapManifests)
	ownedManifests, err := copyDirectory(manifestsDir, b.podManifestPath, false /* overwrite */)
	b.ownedManifests = ownedManifests // always copy in case of partial failure.
	return err
}

// Teardown brings down the bootstrap control plane and cleans up the temporary manifests and secrets and scrape all bootstrap container logs.
// This function is idempotent.
func (b *bootstrapControlPlane) Teardown() {
	UserOutput("Tearing down temporary bootstrap control plane...\n")
	// Copy all CRI-O container logs to systemd
	b.copyContainerLogsToSystemD()

	// Remove all manifests (only after we copied the logs, otherwise kubelet will remove the container and the log file.
	defer b.removeManifests()
}

// TeardownError return aggregated errors from the teardown process.
func (b *bootstrapControlPlane) TeardownError() error {
	messages := []string{}
	for _, err := range b.teardownErrors {
		messages = append(messages, err.Error())
	}
	return fmt.Errorf(strings.Join(messages, "\n"))
}

const podLogsDir = "/var/log/pods"

// listContainerLogFilesForPod lists all log files (0.log, 1.log, etc..) for a given pod UUID.
func listContainerLogFilesForPod(podUUID string) (map[string][]string, error) {
	result := map[string][]string{}
	walkErr := filepath.Walk(filepath.Join(podLogsDir, podUUID), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		containerName := info.Name()
		result[containerName] = []string{}
		// We have container directory, now merge all its *.log files
		if err := filepath.Walk(filepath.Join(path, containerName), func(containerLogPath string, containerLogInfo os.FileInfo, containerDirErr error) error {
			if info.IsDir() {
				return nil
			}
			// Find all *.log files for given containers (more files means container restarts)
			if strings.HasSuffix(info.Name(), ".log") {
				result[containerName] = append(result[containerName], filepath.Join(containerLogPath, info.Name()))
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	})
	return result, walkErr
}

// copyContainerLogsToSystemD copies all containers logs into systemd journal.
func (b *bootstrapControlPlane) copyContainerLogsToSystemD() {
	walkErr := filepath.Walk(podLogsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		podUUID := info.Name()
		containerLogs, err := listContainerLogFilesForPod(podUUID)
		if err != nil {
			return err
		}
		for containerName, logFiles := range containerLogs {
			for containerID, logFile := range logFiles {
				_, err := exec.Command("/bin/systemd-run", "--unit", fmt.Sprintf("bootstrap-%s-%s-%d", podUUID, containerName, containerID), "/bin/cat", logFile).Output()
				if err != nil {
					b.teardownErrors = append(b.teardownErrors, err)
					continue
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		b.teardownErrors = append(b.teardownErrors, walkErr)
	}
}

func (b *bootstrapControlPlane) removeManifests() {
	if err := os.RemoveAll(bootstrapSecretsDir); err != nil {
		b.teardownErrors = append(b.teardownErrors, err)
		return
	}
	for _, manifest := range b.ownedManifests {
		if err := os.Remove(manifest); err != nil && !os.IsNotExist(err) {
			b.teardownErrors = append(b.teardownErrors, err)
		}
	}
	b.ownedManifests = nil
}

// copyFile copies a single file from src to dst. Returns an error if overwrite is true and dst
// exists, or if any I/O error occurs during copying.
func copyFile(src, dst string, overwrite bool) error {
	flags := os.O_CREATE | os.O_WRONLY
	if !overwrite {
		flags |= os.O_EXCL
	}

	dstfile, err := os.OpenFile(dst, flags, os.FileMode(0600))
	if err != nil {
		return err
	}
	defer dstfile.Close()

	srcfile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	_, err = io.Copy(dstfile, srcfile)
	return err
}

// copyDirectory copies srcDir to dstDir recursively. It returns the paths of files (not
// directories) that were copied.
func copyDirectory(srcDir, dstDir string, overwrite bool) ([]string, error) {
	var copied []string
	return copied, filepath.Walk(srcDir, func(src string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, strings.TrimPrefix(src, srcDir))
		if info.IsDir() {
			err = os.Mkdir(dst, os.FileMode(0700))
			if os.IsExist(err) {
				err = nil
			}
			return err
		}
		if err := copyFile(src, dst, overwrite); err != nil {
			return err
		}
		copied = append(copied, dst)
		return nil
	})
}

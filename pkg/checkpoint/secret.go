package checkpoint

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

// checkpointSecretVolumes ensures that all pod secrets are checkpointed locally, then converts the secret volume to a hostpath.
func (c *checkpointer) checkpointSecretVolumes(pod *v1.Pod) (*v1.Pod, error) {
	for i := range pod.Spec.Volumes {
		v := &pod.Spec.Volumes[i]
		if v.Secret == nil {
			continue
		}

		_, err := c.checkpointSecret(pod.Namespace, pod.Name, v.Secret.SecretName)
		if err != nil {
			return nil, fmt.Errorf("failed to checkpoint secret for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
	return pod, nil
}

// checkpointSecret will locally store secret data.
// The path to the secret data becomes: checkpointSecretPath/namespace/podname/secretName/secret.file
// Where each "secret.file" is a key from the secret.Data field.
func (c *checkpointer) checkpointSecret(namespace, podName, secretName string) (string, error) {
	secret, err := c.apiserver.Core().Secrets(namespace).Get(secretName, metav1.GetOptions{})
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

func secretPath(namespace, podName, secretName string) string {
	return filepath.Join(checkpointSecretPath, namespace, podName, secretName)
}

func podFullNameToSecretPath(id string) string {
	namespace, podname := path.Split(id)
	return filepath.Join(checkpointSecretPath, namespace, podname)
}

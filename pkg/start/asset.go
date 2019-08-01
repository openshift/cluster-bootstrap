package start

const (
	assetPathSecrets            = "tls"
	assetPathAdminKubeConfig    = "auth/kubeconfig-loopback"
	assetPathManifests          = "manifests"
	assetPathBootstrapManifests = "bootstrap-manifests"
)

var (
	bootstrapSecretsDir = "/etc/kubernetes/bootstrap-secrets" // Overridden for testing.
)

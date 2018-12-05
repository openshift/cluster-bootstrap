#!/usr/bin/env bash
set -e

mkdir --parents /etc/kubernetes/{manifests,bootstrap-configs,bootstrap-manifests}

KUBE_APISERVER_OPERATOR_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-kube-apiserver-operator)
KUBE_CONTROLLER_MANAGER_OPERATOR_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-kube-controller-manager-operator)
KUBE_SCHEDULER_OPERATOR_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-kube-scheduler-operator)

OPENSHIFT_HYPERSHIFT_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image hypershift)
OPENSHIFT_HYPERKUBE_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image hyperkube)

CLUSTER_BOOTSTRAP_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-bootstrap)

mkdir --parents ./{bootstrap-manifests,manifests}

if [ ! -d cvo-bootstrap ]
then
	echo "Rendering Cluster Version Operator Manifests..."
 
	# shellcheck disable=SC2154
	podman run \
		--volume "$PWD:/assets:z" \
		"{{.ReleaseImageDigest}}" \
		render \
			--output-dir=/assets/cvo-bootstrap \
			--release-image="{{.ReleaseImageDigest}}"

	cp cvo-bootstrap/bootstrap/* bootstrap-manifests/
	cp cvo-bootstrap/manifests/* manifests/
fi

if [ ! -d kube-apiserver-bootstrap ]
then
	echo "Rendering Kubernetes API server core manifests..."

	# shellcheck disable=SC2154
	podman run \
		--volume "$PWD:/assets:z" \
		"${KUBE_APISERVER_OPERATOR_IMAGE}" \
		/usr/bin/cluster-kube-apiserver-operator render \
		--manifest-etcd-serving-ca=etcd-client-ca.crt \
		--manifest-etcd-server-urls={{.EtcdCluster}} \
		--manifest-image=${OPENSHIFT_HYPERSHIFT_IMAGE} \
		--asset-input-dir=/assets/tls \
		--asset-output-dir=/assets/kube-apiserver-bootstrap \
		--config-output-file=/assets/kube-apiserver-bootstrap/config \
		--config-override-files=/assets/bootkube-config-overrides/kube-apiserver-config-overrides.yaml \
		--cluster-config-file=/assets/tectonic/99_openshift-cluster-api_cluster.yaml

	cp kube-apiserver-bootstrap/config /etc/kubernetes/bootstrap-configs/kube-apiserver-config.yaml
	cp kube-apiserver-bootstrap/bootstrap-manifests/* bootstrap-manifests/
	cp kube-apiserver-bootstrap/manifests/* manifests/
fi

if [ ! -d kube-controller-manager-bootstrap ]
then
	echo "Rendering Kubernetes Controller Manager core manifests..."

	# shellcheck disable=SC2154
	podman run \
		--volume "$PWD:/assets:z" \
		"${KUBE_CONTROLLER_MANAGER_OPERATOR_IMAGE}" \
		/usr/bin/cluster-kube-controller-manager-operator render \
		--manifest-image=${OPENSHIFT_HYPERKUBE_IMAGE} \
		--asset-input-dir=/assets/tls \
		--asset-output-dir=/assets/kube-controller-manager-bootstrap \
		--config-output-file=/assets/kube-controller-manager-bootstrap/config \
		--config-override-files=/assets/bootkube-config-overrides/kube-controller-manager-config-overrides.yaml \
		--cluster-config-file=/assets/tectonic/99_openshift-cluster-api_cluster.yaml

	cp kube-controller-manager-bootstrap/config /etc/kubernetes/bootstrap-configs/kube-controller-manager-config.yaml
	cp kube-controller-manager-bootstrap/bootstrap-manifests/* bootstrap-manifests/
	cp kube-controller-manager-bootstrap/manifests/* manifests/
fi

if [ ! -d kube-scheduler-bootstrap ]
then
	echo "Rendering Kubernetes Scheduler core manifests..."

	# shellcheck disable=SC2154
	podman run \
		--volume "$PWD:/assets:z" \
		"${KUBE_SCHEDULER_OPERATOR_IMAGE}" \
		/usr/bin/cluster-kube-scheduler-operator render \
		--manifest-image=${OPENSHIFT_HYPERKUBE_IMAGE} \
		--asset-input-dir=/assets/tls \
		--asset-output-dir=/assets/kube-scheduler-bootstrap \
		--config-output-file=/assets/kube-scheduler-bootstrap/config \
		--config-override-files=/assets/bootkube-config-overrides/kube-scheduler-config-overrides.yaml \
		--disable-phase-2

	cp kube-scheduler-bootstrap/config /etc/kubernetes/bootstrap-configs/kube-scheduler-config.yaml
	cp kube-scheduler-bootstrap/bootstrap-manifests/* bootstrap-manifests/
	cp kube-scheduler-bootstrap/manifests/* manifests/
fi

echo "Starting cluster-bootstrap..."

# shellcheck disable=SC2154
podman run \
	--rm \
	--volume "$PWD:/assets:z" \
	--volume /etc/kubernetes:/etc/kubernetes:z \
	--network=host \
	"${CLUSTER_BOOTSTRAP_IMAGE}" \
	start --asset-dir=/assets --required-pods openshift-kube-apiserver/openshift-kube-apiserver,openshift-kube-scheduler/openshift-kube-scheduler,openshift-kube-controller-manager/openshift-kube-controller-manager,openshift-cluster-version/cluster-version-operator


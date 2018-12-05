#!/usr/bin/env bash
set -e

OUT_DIR=$(mktemp -t cluster-bootstrap -d) # intentionally not deleted for easier debugging
mkdir -p ${OUT_DIR}/{configs,bootstrap-manifests,manifests}

KUBE_APISERVER_OPERATOR_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-kube-apiserver-operator)
KUBE_CONTROLLER_MANAGER_OPERATOR_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-kube-controller-manager-operator)
KUBE_SCHEDULER_OPERATOR_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-kube-scheduler-operator)
CLUSTER_BOOTSTRAP_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image cluster-bootstrap)

OPENSHIFT_HYPERSHIFT_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image hypershift)
OPENSHIFT_HYPERKUBE_IMAGE=$(podman run --rm {{.ReleaseImageDigest}} image hyperkube)

echo "Rendering Cluster Version Operator Manifests..."
# shellcheck disable=SC2154
podman run \
	--volume "{{.AssetsDir}}:/assets:z" \
	--volume "/etc/kubernetes:/etc/kubernetes:z" \
	"{{.ReleaseImageDigest}}" \
	render \
	--output-dir=/etc/kubernetes \
    --release-image="{{.ReleaseImageDigest}}"

echo "Rendering Kubernetes API server core manifests..."
podman run \
	--volume "{{.AssetsDir}}:/assets:z" \
	--volume "${OUT_DIR}:/output:z" \
	"${KUBE_APISERVER_OPERATOR_IMAGE}" \
	/usr/bin/cluster-kube-apiserver-operator render \
	--manifest-etcd-serving-ca=etcd-client-ca.crt \
	--manifest-etcd-server-urls={{.EtcdCluster}} \
	--manifest-image=${OPENSHIFT_HYPERSHIFT_IMAGE} \
	--asset-input-dir=/assets/tls \
	--asset-output-dir=${OUT_DIR} \
	--config-output-file=${OUT_DIR}/configs \
	--config-override-files=/assets/bootkube-config-overrides/kube-apiserver-config-overrides.yaml \
	--cluster-config-file=/assets/tectonic/99_openshift-cluster-api_cluster.yaml

echo "Rendering Kubernetes Controller Manager core manifests..."
podman run \
	--volume "{{.AssetsDir}}:/assets:z" \
	--volume "${OUT_DIR}:/output:z" \
	"${KUBE_CONTROLLER_MANAGER_OPERATOR_IMAGE}" \
	/usr/bin/cluster-kube-controller-manager-operator render \
	--manifest-image=${OPENSHIFT_HYPERKUBE_IMAGE} \
	--asset-input-dir=/assets/tls \
	--asset-output-dir=${OUT_DIR} \
	--config-output-file=${OUT_DIR}/configs \
	--config-override-files=/assets/bootkube-config-overrides/kube-controller-manager-config-overrides.yaml \
	--cluster-config-file=/assets/tectonic/99_openshift-cluster-api_cluster.yaml

echo "Rendering Kubernetes Scheduler core manifests..."
podman run \
	--volume "{{.AssetsDir}}:/assets:z" \
	--volume "${OUT_DIR}:/output:z" \
	"${KUBE_SCHEDULER_OPERATOR_IMAGE}" \
	/usr/bin/cluster-kube-scheduler-operator render \
	--manifest-image=${OPENSHIFT_HYPERKUBE_IMAGE} \
	--asset-input-dir=/assets/tls \
	--asset-output-dir=${OUT_DIR} \
	--config-output-file=${OUT_DIR}/configs \
	--config-override-files=/assets/bootkube-config-overrides/kube-scheduler-config-overrides.yaml \

echo "Starting cluster-bootstrap..."
podman run \
	--rm \
	--volume "${OUT_DIR}:/assets:z" \
	--volume /etc/kubernetes:/etc/kubernetes:z \
	--network=host \
	"${CLUSTER_BOOTSTRAP_IMAGE}" \
	start --asset-dir=/assets --required-pods openshift-kube-apiserver/openshift-kube-apiserver,openshift-kube-scheduler/openshift-kube-scheduler,openshift-kube-controller-manager/openshift-kube-controller-manager,openshift-cluster-version/cluster-version-operator

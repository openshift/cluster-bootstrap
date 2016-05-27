#!/bin/bash
set -euo pipefail

CONFORMANCE_VERSION=${CONFORMANCE_VERSION:-v1.2.4}

usage() {
    echo "USAGE:"
    echo "  $0 <ssh-host> <ssh-port> <ssh-key>"
    echo
    exit 1
}

if [ $# -ne 3 ]; then
    usage
    exit 1
fi

ssh_host=$1
ssh_port=$2
ssh_key=$3

ssh -q -o stricthostkeychecking=no -i ${ssh_key} -p ${ssh_port} core@${ssh_host} \
    "mkdir -p /home/core/go/src/k8s.io/kubernetes && git clone https://github.com/kubernetes/kubernetes /home/core/go/src/k8s.io/kubernetes"

RKT_OPTS=$(echo \
    "--volume=kc,kind=host,source=/home/core/cluster/auth/kubeconfig "\
    "--volume=k8s,kind=host,source=/home/core/go/src/k8s.io/kubernetes " \
    "--mount volume=kc,target=/kubeconfig " \
    "--mount volume=k8s,target=/go/src/k8s.io/kubernetes")

# Init steps necessary to run conformance in docker://golang:1.6.2 container
INIT="apt-get update && apt-get install -y rsync"

CONFORMANCE=$(echo \
    "cd /go/src/k8s.io/kubernetes && " \
    "git checkout ${CONFORMANCE_VERSION} && " \
    "make all WHAT=cmd/kubectl && " \
    "make all WHAT=github.com/onsi/ginkgo/ginkgo && " \
    "make all WHAT=test/e2e/e2e.test && " \
    "KUBECONFIG=/kubeconfig KUBERNETES_CONFORMANCE_TEST=Y hack/ginkgo-e2e.sh -ginkgo.focus='\[Conformance\]'")

CMD="sudo rkt run --insecure-options=image ${RKT_OPTS} docker://golang:1.6.2 --exec /bin/bash -- -c \"${INIT} && ${CONFORMANCE}\""

ssh -q -o stricthostkeychecking=no -i ${ssh_key} -p ${ssh_port} core@${ssh_host} "${CMD}"

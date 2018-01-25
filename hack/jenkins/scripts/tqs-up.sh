#!/usr/bin/env bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
set -x
set -euo pipefail

export TERRAFORM="terraform"
export NUM_WORKERS=${NUM_WORKERS:-1}
export ADDITIONAL_MASTERS=${ADDITIONAL_MASTER:-0}
export REGION="${REGION:-"us-west-2"}"
export CLUSTER_NAME="${CLUSTER_NAME:-"default"}"
export IDENT="${IDENT:-"${HOME}/.ssh/id_rsa"}"

cd "${DIR}/../../terraform-quickstart"

if [[ ! -f "${IDENT}" ]]; then
  mkdir -p "$(dirname "${IDENT}")"
  ssh-keygen -t rsa -f "${IDENT}" -q -N ""
fi

if [ -z "${SSH_AUTH_SOCK:-}" ] ; then
  ssh-agent -s > "/tmp/bootkube-tqs-sshagent.env"
  source "/tmp/bootkube-tqs-sshagent.env"
  ssh-add "${IDENT}"
fi

export TF_VAR_access_key_id="${ACCESS_KEY_ID}"
export TF_VAR_access_key="${ACCESS_KEY_SECRET}"
export TF_VAR_resource_owner="${CLUSTER_NAME}"
export TF_VAR_ssh_public_key="$(cat "${IDENT}.pub")"
export TF_VAR_additional_masters="${ADDITIONAL_MASTERS}"
export TF_VAR_num_workers=${NUM_WORKERS}
export TF_VAR_region="${REGION}"

# bring up compute
"${TERRAFORM}" init
"${TERRAFORM}" apply --auto-approve

# sleep so ssh works with start-cluster
sleep 30

#avoid some IPs being blank bootkube/issues/552
"${TERRAFORM}" refresh

#launch bootkube via quickstart scripts
./start-cluster.sh

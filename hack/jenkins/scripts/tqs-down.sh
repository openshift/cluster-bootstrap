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

export TF_VAR_access_key_id="${ACCESS_KEY_ID}"
export TF_VAR_access_key="${ACCESS_KEY_SECRET}"
export TF_VAR_resource_owner="${CLUSTER_NAME}"
export TF_VAR_ssh_public_key="$(cat "${IDENT}.pub")"
export TF_VAR_additional_masters="${ADDITIONAL_MASTERS}"
export TF_VAR_num_workers=${NUM_WORKERS}
export TF_VAR_region="${REGION}"

# early exit if there's no state file. we probably failed before we even got to terraform
if [[ ! -f "./terraform.tfstate" ]]; then exit 0; fi

for i in 1 2 3 4 5; do
    "${TERRAFORM}" destroy --force && break || sleep 15;
done

# TODO: remove if unnecessary
# if additional resources are destroyed in subsequent calls, the "cleanup" stage will painlessly fail
# giving us an indicator we need this. if we don't, let's remove it.
destroyed_extra=""
for i in 1 2 3; do
    #sleep 30
    output="$("${TERRAFORM}" destroy --force | tail -1)"
    count="$(echo "$output" | sed 's/.*\([0-9]\+\) destroyed.*/\1/')"
    if (( count > 0 )); then
        destroyed_extra="y"
    fi
done

if [[ ! -z "${destroyed_extra:-}" ]]; then
    echo "Terraform required multiple 'destroy' runs to cleanup everything!"
    exit -1
fi

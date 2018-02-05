#!/usr/bin/env bash
set -euo pipefail

export WORKER_IPS=`terraform output -json worker_ips | jq -r '.value[]'`
export MASTER_IPS=`terraform output -json master_ips | jq -r '.value[]'`
export BOOTSTRAP_IP=`terraform output bootstrap_node_ip`
export LOGS_DIR=${LOGS_DIR:-/tmp/logs}
export REMOTE_PORT=${REMOTE_PORT:-22}
export REMOTE_USER=${REMOTE_USER:-core}
export SSH_OPTS=${SSH_OPTS:-}" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

IPS="$WORKER_IPS $MASTER_IPS $BOOTSTRAP_IP"
for IP in $IPS; do
  echo Generating Log for $IP
  ssh -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${IP} 'bash -s' < ../scripts/gatherlogs
done

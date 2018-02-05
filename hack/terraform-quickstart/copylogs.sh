#!/usr/bin/env bash
set -euo pipefail

export WORKER_IPS=`terraform output -json worker_ips | jq -r '.value[]'`
export MASTER_IPS=`terraform output -json master_ips | jq -r '.value[]'`
export BOOTSTRAP_IP=`terraform output bootstrap_node_ip`
export LOGS_DIR=${LOGS_DIR:-$PWD/logs}
export SSH_OPTS=${SSH_OPTS:-}" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

cd ../quickstart

IPS="$WORKER_IPS $MASTER_IPS $BOOTSTRAP_IP"
for IP in $IPS; do
  echo Copying Log for $IP
  ./copylogs.sh $IP
done

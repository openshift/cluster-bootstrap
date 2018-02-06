#!/usr/bin/env bash
set -euo pipefail

export WORKER_IPS=`terraform output -json worker_ips | jq -r '.value[]'`
export MASTER_IPS=`terraform output -json master_ips | jq -r '.value[]'`
export BOOTSTRAP_IP=`terraform output bootstrap_node_ip`
export LOGS_DIR=${LOGS_DIR:-$PWD/logs}
export SSH_OPTS=${SSH_OPTS:-}" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

cd ../quickstart

count=0
for IP in $WORKER_IPS; do
  echo Copying Log for worker $IP
  ./copylogs.sh $IP worker-$count
  count=$((count+1))
done

count=0
for IP in $MASTER_IPS; do
  echo Copying Log for master $IP
  ./copylogs.sh $IP master-$count
  count=$((count+1))
done

count=0
for IP in $BOOTSTRAP_IP; do
  echo Copying Log for master $IP
  ./copylogs.sh $IP bootstrap-$count
  count=$((count+1))
done

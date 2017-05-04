#!/bin/bash
set -euo pipefail

export BOOTSTRAP_IP=`terraform output bootstrap_node_ip`
export WORKER_IPS=`terraform output -json worker_ips | jq -r '.value[]'`
export SELF_HOST_ETCD=`terraform output self_host_etcd`
export SSH_OPTS="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

cd ../quickstart
./init-master.sh $BOOTSTRAP_IP

for IP in $WORKER_IPS
do
  ./init-worker.sh $IP cluster/auth/kubeconfig
done

#!/usr/bin/env bash
set -euo pipefail

# copies logs from the remote machine to a local temporary directory

REMOTE_HOST=$1
NAME=$2
REMOTE_PORT=${REMOTE_PORT:-22}
REMOTE_USER=${REMOTE_USER:-core}
SSH_OPTS=${SSH_OPTS:-}" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"
REMOTE_LOGS_DIR=${REMOTE_LOGS_DIR:-logs}

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host> <name>"
    exit 1
}

[ "$#" == 2 ] || usage

scp -P ${REMOTE_PORT} ${SSH_OPTS} -r ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_LOGS_DIR} .
mv ${REMOTE_LOGS_DIR} logs-${NAME}

echo DONE

#!/usr/bin/env bash
set -euo pipefail

# copies over the gatherlogs script and runs the script on a remote host

REMOTE_HOST=$1
REMOTE_PORT=${REMOTE_PORT:-22}
REMOTE_USER=${REMOTE_USER:-core}
SSH_OPTS=${SSH_OPTS:-}
LOGS_DIR=${LOGS_DIR:-/tmp/logs}

function usage() {
    echo "USAGE:"
    echo "$0: <remote-host>"
    exit 1
}

[ "$#" == 1 ] || usage

if [ "${REMOTE_HOST}" != "local" ]; then
    ssh -p ${REMOTE_PORT} ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} "sudo LOGS_DIR=${LOGS_DIR} REMOTE_USER=${REMOTE_USER}" 'bash -s' < ../scripts/gatherlogs
fi

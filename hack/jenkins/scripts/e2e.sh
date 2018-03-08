#!/usr/bin/env bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
set -x
set -euo pipefail

cd "${DIR}/../../../e2e"

export KUBECONFIG="${KUBECONFIG:-"${DIR}/../../quickstart/cluster/auth/kubeconfig"}"

go test -v -timeout 45m \
  --kubeconfig="${KUBECONFIG}" \
  --keypath="${IDENT}" \
  --expectedmasters=1 \
  ./e2e/

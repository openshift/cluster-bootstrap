## Bootkube E2E Testing

This is the beginnings of E2E testing for the bootkube repo using the standard go testing harness. To run the tests once you have a kubeconfig to a running cluster just execute:
`go test -v ./e2e/ --kubeconfig=<filepath>`

The number of nodes is required so that the setup can block on all nodes being registered.

## Roadmap

Implement whatever is needed to finish porting all functionality from pluton tests

## Requirements

Tests can't rely on network access to the cluster except via the kubernetes api. So no hitting nodes directly just because you can. This will maximize future portability with other setup tools.



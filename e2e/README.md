## Bootkube E2E Testing

This is the beginnings of E2E testing for the bootkube repo using the standard go testing harness. To run the tests once you have a kubeconfig to a running cluster just execute:
`go test -v ./e2e/ --kubeconfig=<filepath>`

The number of nodes is required so that the setup can block on all nodes being registered.

## Roadmap

Implement whatever is needed to finish porting all functionality from pluton tests


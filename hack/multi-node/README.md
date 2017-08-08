# Hack / Dev multi-node build

**Note: All scripts are assumed to be ran from this directory.**

## Quickstart

This will generate the default assets in the `cluster` directory and launch multi-node self-hosted cluster.

```
./bootkube-up
```

## Running E2E tests

```
$ ssh-add ~/.vagrant.d/insecure_private_key
$ export KUBECONFIG=$PWD/cluster/auth/kubeconfig
$ cd ../.. # project root
$ go test -v ./e2e/ --kubeconfig=$KUBECONFIG
```

## Cleaning up

To stop the running cluster and remove generated assets, run:

```
vagrant destroy -f
rm -rf cluster
```

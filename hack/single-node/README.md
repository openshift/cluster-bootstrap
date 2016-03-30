# (wip) Hack / Dev build

## Generate assets

```
bootkube render --outdir=cluster
```

## Add kubeconfig to user-data

```
cat user-data.sample > user-data && sed 's/^/      /' cluster/auth/kubeconfig.yaml >> user-data
```

## Start VM

```
vagrant up
```

## Get SSH info

```
vagrant ssh-config
```

Make note of:

`HostName`
`User`
`Port`
`IdentityFile`

## Start Bootkube

Replace $x with values from above.

```
bootkube start \
  --remote-address=$HostName:$Port \
  --remote-etcd-address=127.0.0.1:2379 \
  --ssh-keyfile=$IdentityFile \
  --ssh-user=$User \
  --manifest-dir=cluster/manifests \
  --apiserver-key=cluster/tls/apiserver.key \
  --apiserver-cert=cluster/tls/apiserver.crt \
  --ca-cert=cluster/tls/ca.crt \
  --token-auth-file=cluster/auth/token-auth.csv \
  --service-account-key=cluster/tls/service-account.key
```

## Inspect node state

```
vagrant ssh
journalctl -lfu kubelet
```

Once kube-apiserver pod started, can manually kill bootkube

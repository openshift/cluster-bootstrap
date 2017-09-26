# Upgrading self-hosted Kubernetes

"Self-hosted" Kubernetes clusters run the apiserver, scheduler, controller-manager, kube-dns, kube-proxy, and flannel or calico as pods, like ordinary applications. This allows upgrades to be performed in-place using (mostly) `kubectl`, as an alternative to re-provisioning.

Let's upgrade a Kubernetes v1.6.6 cluster to v1.6.7 as an example.

## Status

The process of in-place upgrading a self-hosted Kubernetes cluster can be straight-forward, but there may be complex underlying changes that affect your cluster. In most cases, patch version upgrades (e.g. v1.6.6 to v1.6.7) are safe.

Before beginning an upgrade, it is recommended you evaluate if an in-place upgrade is appropriate for your Kubernetes cluster and risk tolerance. You may wish to test the in-place upgrade on a development cluster, failover important workloads to another cluster, or be prepared to handle unforseen issues.

## Prepare

Find the diff between bootkube assets generated for the existing cluster version and the desired version. This depends on the tool used to generate assets:

* Github [compare](https://github.com/kubernetes-incubator/bootkube/compare/v0.5.0...v0.5.1) changes between the existing and desired versions and infer the appropriate cluster changes.
* [bootkube render](https://github.com/kubernetes-incubator/bootkube) - Install the `bootkube` binaries for the existing and desired versions. Render assets to different locations with each binary and diff the assets.
* [External Tools](users-integrations.md) - Check the docs for the external tool and compare assets generated for each version.

In simple cases, you may only need to bump the version of a few images. In more complex cases, there may be entirely new components, configuration, or flags.

## Inspect

Check the current Kubernetes version.

```sh
$ kubectl version
Client Version: version.Info{Major:"1", Minor:"6", GitVersion:"v1.6.2", GitCommit:"477efc3cbe6a7effca06bd1452fa356e2201e1ee", GitTreeState:"clean", BuildDate:"2017-04-19T20:33:11Z", GoVersion:"go1.7.5", Compiler:"gc", Platform:"linux/amd64"}
Server Version: version.Info{Major:"1", Minor:"6", GitVersion:"v1.6.6+coreos.1", GitCommit:"42a5c8b99c994a51d9ceaed5d0254f177e97d419", GitTreeState:"clean", BuildDate:"2017-06-21T01:10:07Z", GoVersion:"go1.7.6", Compiler:"gc", Platform:"linux/amd64"}
```

```sh
$ kubectl get nodes
NAME                               STATUS    AGE       VERSION
node1.example.com                  Ready     21d       v1.6.6+coreos.1
node2.example.com                  Ready     21d       v1.6.6+coreos.1
node3.example.com                  Ready     21d       v1.6.6+coreos.1
node4.example.com                  Ready     21d       v1.6.6+coreos.1
```

## Control Plane

Show the control plane DaemonSets and Deployments that will need to be updated.

```sh
$ kubectl get daemonsets -n=kube-system
NAME                             DESIRED   CURRENT   READY     UP-TO-DATE   AVAILABLE   NODE-SELECTOR                     AGE
kube-apiserver                   1         1         1         1            1           node-role.kubernetes.io/master=   21d
kube-flannel                     4         4         4         4            4           <none>                            21d
kube-proxy                       4         4         4         4            4           <none>                            21d
pod-checkpointer                 1         1         1         1            1           node-role.kubernetes.io/master=   21d

$ kubectl get deployments -n=kube-system
kube-controller-manager           2         2         2            2           21d
kube-dns                          1         1         1            1           21d
kube-scheduler                    2         2         2            2           21d
```

### kube-apiserver

If only the container image version has changed, update the image with a single command.

```
kubectl set image daemonset kube-apiserver kube-apiserver=quay.io/coreos/hyperkube:v1.6.7_coreos.0
```

You can edit the daemonset directly if other changes are needed.

```sh
$ kubectl edit daemonset kube-apiserver -n=kube-system
```

With only one apiserver, the cluster may be momentarily unavailable.

### kube-scheduler

Again, if only the container image version has changed, update the image with a single command.

```
kubectl set image deployment kube-scheduler kube-scheduler=quay.io/coreos/hyperkube:v1.6.7_coreos.0
```

You can edit the deployment directly if other changes are needed.

```sh
$ kubectl edit deployments kube-scheduler -n=kube-system
```

### kube-controller-manager

Again, if only the container image version has changed, update the image with a single command.

```
kubectl set image deployment kube-controller-manager kube-controller-manager=quay.io/coreos/hyperkube:v1.6.7_coreos.0
```

You can edit the deployment directly if other changes are needed.

```sh
$ kubectl edit deployments kube-controller-manager -n=kube-system
```

### kube-proxy

Edit the `kube-proxy` daemonset to rolling update the proxy.

```sh
$ kubectl edit daemonset kube-proxy -n=kube-system
```

### Others

Update any other components which have changes between the existing version and desired version manifests. Update the `kube-dns` deployment, `kube-flannel` daemonset, or `pod-checkpointer` daemonset.

### Verify

Verify the control plane components updated.

```sh
$ kubectl version
Client Version: version.Info{Major:"1", Minor:"6", GitVersion:"v1.6.2", GitCommit:"477efc3cbe6a7effca06bd1452fa356e2201e1ee", GitTreeState:"clean", BuildDate:"2017-04-19T20:33:11Z", GoVersion:"go1.7.5", Compiler:"gc", Platform:"linux/amd64"}
Server Version: version.Info{Major:"1", Minor:"6", GitVersion:"v1.6.7+coreos.0", GitCommit:"c8c505ee26ac3ab4d1dff506c46bc5538bc66733", GitTreeState:"clean", BuildDate:"2017-07-06T17:38:33Z", GoVersion:"go1.7.6", Compiler:"gc", Platform:"linux/amd64"}
```

```sh
$ kubectl get nodes
NAME                               STATUS    AGE       VERSION
node1.example.com                  Ready     21d       v1.6.7+coreos.0
node2.example.com                  Ready     21d       v1.6.7+coreos.0
node3.example.com                  Ready     21d       v1.6.7+coreos.0
node4.example.com                  Ready     21d       v1.6.7+coreos.0
```

## kubelet

SSH to each node and update the `KUBELET_IMAGE_TAG` in `kubelet.service` or `/etc/kubernetes/kubelet.env`, depending on the provisioning tool used. Restart the `kubelet.service`.

```sh
ssh core@node1.example.com
sudo vim /etc/systemd/system/kubelet.service
sudo vim /etc/kubernetes/kubelet.env
sudo systemctl restart kubelet
```

### Verify

Verify the kubelet and kube-proxy of each node updated.

```sh
$ kubectl get nodes -o yaml | grep 'kubeletVersion\|kubeProxyVersion'
      kubeProxyVersion: v1.6.7+coreos.0
      kubeletVersion: v1.6.7+coreos.0
      kubeProxyVersion: v1.6.7+coreos.0
      kubeletVersion: v1.6.7+coreos.0
      kubeProxyVersion: v1.6.7+coreos.0
      kubeletVersion: v1.6.7+coreos.0
      kubeProxyVersion: v1.6.7+coreos.0
      kubeletVersion: v1.6.7+coreos.0
```

Kubernetes control plane components have been successfully updated!


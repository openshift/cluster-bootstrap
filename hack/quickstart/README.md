## GCE Quickstart

### Launch nodes

To find the latest CoreOS alpha/beta/stable images, please see the (CoreOS GCE Documentation)[https://coreos.com/os/docs/latest/booting-on-google-compute-engine.html]

Launch 3 nodes:

```
$ gcloud compute instances create k8s-core1 k8s-core2 k8s-core3 \
  --image https://www.googleapis.com/compute/v1/projects/coreos-cloud/global/images/coreos-alpha-1068-0-0-v20160607 \
  --zone us-central1-a --machine-type n1-standard-1
```

Tag the master node, and allow traffic to 443 on that node.

```
$ gcloud compute instances add-tags k8s-core1 --tags apiserver
$ gcloud compute firewall-rules create api-443 --target-tags=apiserver --allow tcp:443
```

### Bootstrap Master

*Replace* `<node-ip>` with the EXTERNAL_IP from output of `gcloud compute instances list k8s-core1`.

```
$ IDENT=~/.ssh/google_compute_engine ./init-master.sh <node-ip>
```

After the master bootstrap is complete, you can continue to add worker nodes. Or cluster state can be inspected via kubectl:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```

### Bootstrap Nodes

Get the EXTERNAL_IP from each node you wish to add:

```
gcloud compute instances list k8s-core2
gcloud compute instances list k8s-core3
```

Initialize each worker node by replacing `<node-ip>` with the EXTERNAL_IP from the commands above.

```
IDENT=~/.ssh/google_compute_engine ./init-worker.sh <node-ip> cluster/auth/kubeconfig
```

**NOTE:** It can take a few minutes for each node to download all of the required assets / containers.
 They may not be immediately available, but the state can be inspected with:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```

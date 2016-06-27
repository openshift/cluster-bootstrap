## AWS Quickstart

### Configure Security Groups

Make note of the `GroupId` output of this command, as it will be referenced later in this guide as `<K8S_SG_ID>`.

```
$ aws ec2 create-security-group --region us-west-2 --group-name k8s-sg --description "Security group for k8s cluster"
GroupID: "sg-abcdefg"
```

Next, create the security group rules.

```
$ aws ec2 authorize-security-group-ingress --region us-west-2 --group-name k8s-sg --protocol tcp --port 22 --cidr 0.0.0.0/0
$ aws ec2 authorize-security-group-ingress --region us-west-2 --group-name k8s-sg --protocol tcp --port 443 --cidr 0.0.0.0/0
$ aws ec2 authorize-security-group-ingress --region us-west-2 --group-name k8s-sg --protocol tcp --port 0-65535 --source-group k8s-sg
```

### Create a key-pair

```
$ aws ec2 create-key-pair --key-name k8s-key --query 'KeyMaterial' --output text > k8s-key.pem
$ chmod 400 k8s-key.pem
```

### Launch Nodes

To find the latest CoreOS alpha/beta/stable images, please see the [CoreOS AWS Documentation](https://coreos.com/os/docs/latest/booting-on-ec2.html). Then replace the `--image-id` in the command below.

In the command below, replace `<K8S_SG_ID>` with the security-group-id noted earlier.

```
$ aws ec2 run-instances --region us-west-2 --image-id ami-184a8f78 --security-group-ids <K8S_SG_ID> --count 1 --instance-type m3.medium --key-name k8s-key --query 'Instances[0].InstanceId'
"i-abcdefgh"
```

Next we will use the output of the above command (instance-id) in place of <INSTANCE_ID> in the command below:

```
$ aws ec2 describe-instances --region us-west-2 --instance-ids <INSTANCE_ID> --query 'Reservations[0].Instances[0].PublicIpAddress'
```

### Bootstrap Master

We can then use the public-ip to initialize a master node:

```
$ IDENT=k8s-key.pem ./init-master.sh <PUBLIC_IP>
```

After the master bootstrap is complete, you can continue to add worker nodes. Or cluster state can be inspected via kubectl:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```

### Add Workers

Run the `Launch Nodes` step for each additional node you wish to add, then using the public-ip, run:

```
IDENT=k8s-key.pem ./init-worker.sh <PUBLIC_IP> cluster/auth/kubeconfig
```

**NOTE:** It can take a few minutes for each node to download all of the required assets / containers.
 They may not be immediately available, but the state can be inspected with:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```

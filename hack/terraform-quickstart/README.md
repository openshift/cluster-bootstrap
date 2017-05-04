## Terraform-quickstart
This directory provides a basic way to use terraform to setup compute resources on AWS. It was written with testing in mind.

Prerequisites:
 - terraform 
 - bootkube binary built from the repo root

To start a cluster first fill out the terraform.tfvars.example with the needed secrets and rename it to terraform.tfvars. Then:

```
terraform plan
terraform apply
./start-cluster.sh
```

To destroy a cluster:

```
terraform destroy
```

variable "access_key_id" {
  type = "string"
}

variable "access_key" {
  type = "string"
}

variable "ssh_key" {
  description = "aws ssh key"
  type        = "string"
}

variable "instance_tags" {
  description = "Name all instances behind a single tag based on who/what is running terraform"
  type        = "string"
}

variable "self_host_etcd" {
  type    = "string"
  default = "true"
}

variable "num_workers" {
  description = "number of worker nodes"
  type        = "string"
  default     = "1"
}

variable "region" {
  description = "aws region"
  type        = "string"
  default     = "us-east-1"
}

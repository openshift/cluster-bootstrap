provider "aws" {
  access_key = "${var.access_key_id}"
  secret_key = "${var.access_key}"
  region     = "${var.region}"
}

resource "aws_instance" "bootstrap_node" {
  ami                  = "${data.aws_ami.coreos_ami.image_id}"
  instance_type        = "${var.instance_type}"
  key_name             = "${var.ssh_key}"
  iam_instance_profile = "${aws_iam_instance_profile.bk_profile.id}"

  vpc_security_group_ids      = ["${aws_security_group.allow_all.id}"]
  subnet_id                   = "${aws_subnet.main.id}"
  associate_public_ip_address = true
  depends_on                  = ["aws_internet_gateway.main"]

  tags {
    Name = "${var.resource_owner}"
  }

  root_block_device {
    volume_type = "gp2"
    volume_size = "30"
  }
}

resource "aws_instance" "worker_node" {
  ami                  = "${data.aws_ami.coreos_ami.image_id}"
  instance_type        = "${var.instance_type}"
  key_name             = "${var.ssh_key}"
  count                = "${var.num_workers}"
  iam_instance_profile = "${aws_iam_instance_profile.bk_profile.id}"

  vpc_security_group_ids      = ["${aws_security_group.allow_all.id}"]
  subnet_id                   = "${aws_subnet.main.id}"
  associate_public_ip_address = true
  depends_on                  = ["aws_internet_gateway.main"]

  tags {
    Name = "${var.resource_owner}"
  }

  root_block_device {
    volume_type = "gp2"
    volume_size = "30"
  }
}

resource "aws_instance" "master_node" {
  ami                  = "${data.aws_ami.coreos_ami.image_id}"
  instance_type        = "${var.instance_type}"
  key_name             = "${var.ssh_key}"
  count                = "${var.additional_masters}"
  iam_instance_profile = "${aws_iam_instance_profile.bk_profile.id}"

  vpc_security_group_ids      = ["${aws_security_group.allow_all.id}"]
  subnet_id                   = "${aws_subnet.main.id}"
  associate_public_ip_address = true
  depends_on                  = ["aws_internet_gateway.main"]

  tags {
    Name = "${var.resource_owner}"
  }

  root_block_device {
    volume_type = "gp2"
    volume_size = "30"
  }
}

data "aws_ami" "coreos_ami" {
  most_recent = true

  filter {
    name   = "name"
    values = ["CoreOS-stable-*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "owner-id"
    values = ["595879546273"]
  }
}

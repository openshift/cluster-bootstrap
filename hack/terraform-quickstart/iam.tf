resource "aws_iam_instance_profile" "bk_profile" {
  name = "bootkube_profile"
  role = "${aws_iam_role.bk_role.name}"
}

resource "aws_iam_role" "bk_role" {
  name = "bootkube_e2e_role"
  path = "/"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "bk_policy" {
  name = "bootkube_e2e_policy"
  role = "${aws_iam_role.bk_role.id}"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "ec2:Describe*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

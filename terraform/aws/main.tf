# =============================================================================
# terraform/aws/main.tf — AWS Spot instance for system-design project testing
#
# Provisions:
#   - VPC + public subnet + IGW + route table
#   - Security group: SSH, HTTP, HTTPS, app ports (8081-8090), Prometheus, Grafana
#   - Latest Ubuntu 24.04 ARM64 AMI (matches Oracle Cloud dev environment)
#   - t4g.large Spot instance (2 vCPU / 8 GB, ~$0.015/hr in ap-south-1)
#   - EIP so IP survives stop/start
#   - cloud-init: installs Docker, clones repo, deploys all projects
#
# Usage:
#   terraform init
#   terraform plan
#   terraform apply
#   terraform destroy   # when done testing (Spot = pay per second)
# =============================================================================

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.90"
    }
  }
  required_version = ">= 1.15"
}

provider "aws" {
  region = var.aws_region
}

# ── Latest Ubuntu 24.04 ARM64 AMI (official Canonical) ───────────────────────
data "aws_ami" "ubuntu_arm64" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-arm64-server-*"]
  }

  filter {
    name   = "architecture"
    values = ["arm64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# ── Current Spot price data ───────────────────────────────────────────────────
data "aws_ec2_spot_price" "current" {
  instance_type     = var.instance_type
  availability_zone = "${var.aws_region}${var.availability_zone_suffix}"

  filter {
    name   = "product-description"
    values = ["Linux/UNIX"]
  }
}

# =============================================================================
# NETWORK
# =============================================================================

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = merge(local.common_tags, { Name = "${var.project_name}-vpc" })
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.main.id
  tags   = merge(local.common_tags, { Name = "${var.project_name}-igw" })
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "${var.aws_region}${var.availability_zone_suffix}"
  map_public_ip_on_launch = true

  tags = merge(local.common_tags, { Name = "${var.project_name}-public" })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }

  tags = merge(local.common_tags, { Name = "${var.project_name}-rt" })
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# =============================================================================
# SECURITY GROUP
# =============================================================================

resource "aws_security_group" "main" {
  name        = "${var.project_name}-sg"
  description = "SSH, HTTP, HTTPS, app ports, Prometheus, Grafana"
  vpc_id      = aws_vpc.main.id

  # SSH — restrict to your IP via var.ssh_allowed_cidr
  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.ssh_allowed_cidr]
  }

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # App service ports (projects 01–10)
  ingress {
    description = "App ports"
    from_port   = 8081
    to_port     = 8090
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Prometheus
  ingress {
    description = "Prometheus"
    from_port   = 9090
    to_port     = 9090
    protocol    = "tcp"
    cidr_blocks = [var.observability_allowed_cidr]
  }

  # Grafana
  ingress {
    description = "Grafana"
    from_port   = 3000
    to_port     = 3000
    protocol    = "tcp"
    cidr_blocks = [var.observability_allowed_cidr]
  }

  # Node exporter — internal only
  ingress {
    description = "Node Exporter"
    from_port   = 9100
    to_port     = 9100
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "${var.project_name}-sg" })
}

# =============================================================================
# KEY PAIR
# =============================================================================

resource "aws_key_pair" "deployer" {
  key_name   = "${var.project_name}-key"
  public_key = var.ssh_public_key

  tags = local.common_tags
}

# =============================================================================
# SPOT INSTANCE
# =============================================================================

resource "aws_spot_instance_request" "main" {
  ami                    = data.aws_ami.ubuntu_arm64.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.main.id]
  key_name               = aws_key_pair.deployer.key_name
  availability_zone      = "${var.aws_region}${var.availability_zone_suffix}"

  # Bid slightly above current Spot price to avoid immediate interruption.
  # Current Spot in ap-south-1c: ~$0.015/hr; on-demand cap: $0.0448/hr.
  spot_price           = var.spot_price_max
  spot_type            = "persistent"
  wait_for_fulfillment = true

  # On interruption: stop (not terminate) so EBS root survives.
  instance_interruption_behavior = "stop"

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 30
    delete_on_termination = true
    encrypted             = true
  }

  user_data = base64encode(join("\n", [
    "#!/bin/bash",
    "export REPO_URL='${var.repo_url}'",
    "export GITHUB_TOKEN='${var.github_token}'",
    file("${path.module}/cloud-init.sh")
  ]))

  # IMDSv2 required — prevents SSRF attacks from reading instance metadata
  # (including user_data which contains GITHUB_TOKEN) via the metadata endpoint.
  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    http_endpoint               = "enabled"
  }

  tags = merge(local.common_tags, { Name = "${var.project_name}-spot" })

  # Tag the underlying instance (separate from the Spot request tags)
  volume_tags = merge(local.common_tags, { Name = "${var.project_name}-root-vol" })
}

# ── Elastic IP (static public IP) ────────────────────────────────────────────
# Attached to the instance id surfaced by the Spot request after fulfillment.
resource "aws_eip" "main" {
  instance = aws_spot_instance_request.main.spot_instance_id
  domain   = "vpc"

  depends_on = [aws_internet_gateway.igw]
  tags       = merge(local.common_tags, { Name = "${var.project_name}-eip" })
}

# =============================================================================
# LOCALS
# =============================================================================

locals {
  common_tags = {
    Project     = var.project_name
    Environment = "spot-test"
    ManagedBy   = "terraform"
  }
}

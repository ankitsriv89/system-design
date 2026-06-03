# =============================================================================
# terraform/aws-multi/main.tf — Three Spot instances sharing one VPC + infra
#
# Architecture:
#   One VPC, one SG, one shared-infra instance (Postgres + Redis + Kafka +
#   Prometheus + Grafana), plus three project-runner instances grouped by stack:
#
#     go-runner    — t4g.medium  (2 vCPU / 4 GB)  — Go projects
#     java-runner  — t4g.large   (2 vCPU / 8 GB)  — Java/Spring Boot + Kafka
#     other-runner — t4g.medium  (2 vCPU / 4 GB)  — Python / Node.js projects
#
#   All runners reach shared-infra services (Postgres, Redis, Kafka) over the
#   private VPC subnet — no duplication, no extra cost.
#
# Usage:
#   terraform init
#   terraform apply
#   terraform destroy   # destroy all three at once when done
#
# Cost estimate (ap-south-1 Spot, ~2026 prices):
#   go-runner    t4g.medium  ~$0.008/hr
#   java-runner  t4g.large   ~$0.015/hr
#   other-runner t4g.medium  ~$0.008/hr
#   infra        t4g.medium  ~$0.008/hr
#   EIPs (×4)                ~$0.000/hr (free while attached)
#   Total running            ~$0.039/hr (~$0.94/day)
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

# ── Latest Ubuntu 24.04 ARM64 AMI ────────────────────────────────────────────
data "aws_ami" "ubuntu_arm64" {
  most_recent = true
  owners      = ["099720109477"]
  filter { name = "name";                values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-arm64-server-*"] }
  filter { name = "architecture";        values = ["arm64"] }
  filter { name = "virtualization-type"; values = ["hvm"] }
}

# =============================================================================
# NETWORK — one VPC shared by all instances
# =============================================================================

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = merge(local.common_tags, { Name = "system-design-vpc" })
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.main.id
  tags   = merge(local.common_tags, { Name = "system-design-igw" })
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "${var.aws_region}${var.az_suffix}"
  map_public_ip_on_launch = true
  tags                    = merge(local.common_tags, { Name = "system-design-public" })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route { cidr_block = "0.0.0.0/0"; gateway_id = aws_internet_gateway.igw.id }
  tags = merge(local.common_tags, { Name = "system-design-rt" })
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# =============================================================================
# SECURITY GROUP — shared by all instances in the VPC
# =============================================================================

resource "aws_security_group" "main" {
  name        = "system-design-sg"
  description = "SSH, HTTP/S, all app ports 8081-8131, Prometheus, Grafana, internal VPC"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "SSH"
    from_port   = 22; to_port = 22; protocol = "tcp"
    cidr_blocks = [var.ssh_allowed_cidr]
  }
  ingress {
    description = "HTTP (Caddy)"
    from_port   = 80; to_port = 80; protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    description = "HTTPS (Caddy)"
    from_port   = 443; to_port = 443; protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  # All 50 project ports
  ingress {
    description = "App ports p01-p50"
    from_port   = 8081; to_port = 8131; protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    description = "Prometheus"
    from_port   = 9090; to_port = 9090; protocol = "tcp"
    cidr_blocks = [var.observability_allowed_cidr]
  }
  ingress {
    description = "Grafana"
    from_port   = 3000; to_port = 3000; protocol = "tcp"
    cidr_blocks = [var.observability_allowed_cidr]
  }
  # Internal VPC traffic — allows runners to reach shared-infra (Postgres 5432,
  # Redis 6379, Kafka 9092) without exposing those ports to the internet.
  ingress {
    description = "Internal VPC (shared infra ports)"
    from_port   = 0; to_port = 65535; protocol = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
  }
  egress {
    from_port   = 0; to_port = 0; protocol = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "system-design-sg" })
}

# =============================================================================
# KEY PAIR
# =============================================================================

resource "aws_key_pair" "deployer" {
  key_name   = "system-design-multi-key"
  public_key = var.ssh_public_key
  tags       = local.common_tags
}

# =============================================================================
# SHARED INFRA INSTANCE — Postgres + Redis + Kafka (KRaft) + Prometheus + Grafana
#
# t4g.medium (2 vCPU / 4 GB) is enough for idle shared services.
# Kafka is KRaft mode (no Zookeeper) to save ~200 MB.
# =============================================================================

resource "aws_spot_instance_request" "infra" {
  ami                            = data.aws_ami.ubuntu_arm64.id
  instance_type                  = "t4g.medium"
  subnet_id                      = aws_subnet.public.id
  vpc_security_group_ids         = [aws_security_group.main.id]
  key_name                       = aws_key_pair.deployer.key_name
  availability_zone              = "${var.aws_region}${var.az_suffix}"
  spot_price                     = "0.0224" # t4g.medium on-demand ceiling
  spot_type                      = "persistent"
  wait_for_fulfillment           = true
  instance_interruption_behavior = "stop"

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 30
    delete_on_termination = true
    encrypted             = true
  }

  user_data = base64encode(templatefile("${path.module}/cloud-init-infra.sh", {
    repo_url      = var.repo_url
    github_token  = var.github_token
  }))

  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    http_endpoint               = "enabled"
  }

  tags        = merge(local.common_tags, { Name = "system-design-infra", Role = "shared-infra" })
  volume_tags = merge(local.common_tags, { Name = "system-design-infra-vol" })
}

resource "aws_eip" "infra" {
  instance   = aws_spot_instance_request.infra.spot_instance_id
  domain     = "vpc"
  depends_on = [aws_internet_gateway.igw]
  tags       = merge(local.common_tags, { Name = "system-design-infra-eip" })
}

# =============================================================================
# GO RUNNER — all Go projects (p01-p09, p11-p12, p14-p16, p17, p18 …)
# t4g.medium: 2 vCPU / 4 GB — Go binaries idle at 30-80 MB each
# =============================================================================

resource "aws_spot_instance_request" "go_runner" {
  ami                            = data.aws_ami.ubuntu_arm64.id
  instance_type                  = "t4g.medium"
  subnet_id                      = aws_subnet.public.id
  vpc_security_group_ids         = [aws_security_group.main.id]
  key_name                       = aws_key_pair.deployer.key_name
  availability_zone              = "${var.aws_region}${var.az_suffix}"
  spot_price                     = "0.0224"
  spot_type                      = "persistent"
  wait_for_fulfillment           = true
  instance_interruption_behavior = "stop"

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 20
    delete_on_termination = true
    encrypted             = true
  }

  user_data = base64encode(templatefile("${path.module}/cloud-init-runner.sh", {
    repo_url      = var.repo_url
    github_token  = var.github_token
    stack_name    = "go"
    infra_host    = aws_spot_instance_request.infra.private_ip
  }))

  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    http_endpoint               = "enabled"
  }

  tags        = merge(local.common_tags, { Name = "system-design-go-runner", Role = "go-projects" })
  volume_tags = merge(local.common_tags, { Name = "system-design-go-vol" })
}

resource "aws_eip" "go_runner" {
  instance   = aws_spot_instance_request.go_runner.spot_instance_id
  domain     = "vpc"
  depends_on = [aws_internet_gateway.igw]
  tags       = merge(local.common_tags, { Name = "system-design-go-eip" })
}

# =============================================================================
# JAVA RUNNER — Java/Spring Boot projects (p10, p13, p22, p39 …)
# t4g.large: 2 vCPU / 8 GB — JVM needs headroom; Kafka client in-process
# =============================================================================

resource "aws_spot_instance_request" "java_runner" {
  ami                            = data.aws_ami.ubuntu_arm64.id
  instance_type                  = "t4g.large"
  subnet_id                      = aws_subnet.public.id
  vpc_security_group_ids         = [aws_security_group.main.id]
  key_name                       = aws_key_pair.deployer.key_name
  availability_zone              = "${var.aws_region}${var.az_suffix}"
  spot_price                     = "0.0448" # t4g.large on-demand ceiling
  spot_type                      = "persistent"
  wait_for_fulfillment           = true
  instance_interruption_behavior = "stop"

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 20
    delete_on_termination = true
    encrypted             = true
  }

  user_data = base64encode(templatefile("${path.module}/cloud-init-runner.sh", {
    repo_url      = var.repo_url
    github_token  = var.github_token
    stack_name    = "java"
    infra_host    = aws_spot_instance_request.infra.private_ip
  }))

  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    http_endpoint               = "enabled"
  }

  tags        = merge(local.common_tags, { Name = "system-design-java-runner", Role = "java-projects" })
  volume_tags = merge(local.common_tags, { Name = "system-design-java-vol" })
}

resource "aws_eip" "java_runner" {
  instance   = aws_spot_instance_request.java_runner.spot_instance_id
  domain     = "vpc"
  depends_on = [aws_internet_gateway.igw]
  tags       = merge(local.common_tags, { Name = "system-design-java-eip" })
}

# =============================================================================
# OTHER RUNNER — Python, Node.js projects (p24 ML, p46 model-serving, etc.)
# t4g.medium: 2 vCPU / 4 GB — most Python/Node services are lightweight
# Upgrade to t4g.large if ML inference projects need more RAM
# =============================================================================

resource "aws_spot_instance_request" "other_runner" {
  ami                            = data.aws_ami.ubuntu_arm64.id
  instance_type                  = "t4g.medium"
  subnet_id                      = aws_subnet.public.id
  vpc_security_group_ids         = [aws_security_group.main.id]
  key_name                       = aws_key_pair.deployer.key_name
  availability_zone              = "${var.aws_region}${var.az_suffix}"
  spot_price                     = "0.0224"
  spot_type                      = "persistent"
  wait_for_fulfillment           = true
  instance_interruption_behavior = "stop"

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 20
    delete_on_termination = true
    encrypted             = true
  }

  user_data = base64encode(templatefile("${path.module}/cloud-init-runner.sh", {
    repo_url      = var.repo_url
    github_token  = var.github_token
    stack_name    = "other"
    infra_host    = aws_spot_instance_request.infra.private_ip
  }))

  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    http_endpoint               = "enabled"
  }

  tags        = merge(local.common_tags, { Name = "system-design-other-runner", Role = "other-projects" })
  volume_tags = merge(local.common_tags, { Name = "system-design-other-vol" })
}

resource "aws_eip" "other_runner" {
  instance   = aws_spot_instance_request.other_runner.spot_instance_id
  domain     = "vpc"
  depends_on = [aws_internet_gateway.igw]
  tags       = merge(local.common_tags, { Name = "system-design-other-eip" })
}

# =============================================================================
# LOCALS
# =============================================================================

locals {
  common_tags = {
    Project     = "system-design"
    Environment = "spot-test"
    ManagedBy   = "terraform"
  }
}

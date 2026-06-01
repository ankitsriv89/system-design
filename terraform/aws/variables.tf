# =============================================================================
# terraform/aws/variables.tf — Input variables for AWS Spot deployment
#
# Set values in terraform.tfvars (gitignored). Never commit secrets.
# =============================================================================

variable "aws_region" {
  description = "AWS region to deploy into. ap-south-1 (Mumbai) is cheapest for ARM Spot."
  type        = string
  default     = "ap-south-1"
}

variable "availability_zone_suffix" {
  description = "AZ suffix appended to aws_region. 'c' is typically cheapest in ap-south-1."
  type        = string
  default     = "c"
}

variable "instance_type" {
  description = "EC2 instance type. t4g.large = 2 vCPU / 8 GB ARM64, ~$0.015/hr Spot."
  type        = string
  default     = "t4g.large"
}

variable "spot_price_max" {
  description = "Maximum Spot bid price ($/hr). Set to on-demand price as ceiling so it never exceeds that."
  type        = string
  default     = "0.0448" # t4g.large on-demand in ap-south-1
}

variable "project_name" {
  description = "Prefix applied to all resource names and tags."
  type        = string
  default     = "system-design"
}

variable "ssh_public_key" {
  description = "Contents of your SSH public key (e.g. ~/.ssh/id_ed25519.pub). Used to create an EC2 key pair."
  type        = string
  sensitive   = true
}

variable "ssh_allowed_cidr" {
  description = "CIDR allowed to reach port 22. Use your home IP /32 for security."
  type        = string
  default     = "0.0.0.0/0"
}

variable "observability_allowed_cidr" {
  description = "CIDR allowed to reach Prometheus (:9090) and Grafana (:3000). Defaults to your IP only."
  type        = string
  default     = "0.0.0.0/0"
}

variable "repo_url" {
  description = "Git repository URL to clone on first boot."
  type        = string
  default     = "https://github.com/ankitsriv89/system-design.git"
}

variable "github_token" {
  description = "Optional GitHub personal access token for private repos. Leave empty for public repos."
  type        = string
  default     = ""
  sensitive   = true
}

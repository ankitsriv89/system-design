# =============================================================================
# terraform/aws-multi/variables.tf
# =============================================================================

variable "aws_region" {
  description = "AWS region. ap-south-1 (Mumbai) cheapest ARM Spot."
  type        = string
  default     = "ap-south-1"
}

variable "az_suffix" {
  description = "AZ suffix. 'c' is typically cheapest in ap-south-1."
  type        = string
  default     = "c"
}

variable "ssh_public_key" {
  description = "SSH public key contents (~/.ssh/id_ed25519.pub)."
  type        = string
  sensitive   = true
}

variable "ssh_allowed_cidr" {
  description = "CIDR allowed to SSH. Restrict to your home IP /32 for security."
  type        = string
  default     = "0.0.0.0/0"
}

variable "observability_allowed_cidr" {
  description = "CIDR allowed to reach Prometheus (:9090) and Grafana (:3000)."
  type        = string
  default     = "0.0.0.0/0"
}

variable "repo_url" {
  description = "Git repository URL."
  type        = string
  default     = "https://github.com/ankitsriv89/system-design.git"
}

variable "github_token" {
  description = "GitHub PAT for private repos. Leave empty for public."
  type        = string
  default     = ""
  sensitive   = true
}

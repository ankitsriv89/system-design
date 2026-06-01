# =============================================================================
# variables.tf — All input variables for the OCI free-tier deployment
#
# Set values in terraform.tfvars (gitignored) or as Terraform Cloud variables.
# Never commit credentials to the repo.
# =============================================================================

# ── Terraform Cloud ───────────────────────────────────────────────────────────

variable "tfc_organization" {
  description = "Terraform Cloud organization name. Only needed if using the cloud {} backend."
  type        = string
  default     = ""
}

variable "tfc_workspace" {
  description = "Terraform Cloud workspace name"
  type        = string
  default     = "system-design-oci"
}

# ── OCI identity ──────────────────────────────────────────────────────────────

variable "tenancy_ocid" {
  description = "OCID of your OCI tenancy (Profile → Tenancy Information)"
  type        = string
  sensitive   = true
}

variable "compartment_ocid" {
  description = "OCID of the compartment to deploy into (root compartment = tenancy_ocid)"
  type        = string
  sensitive   = true
}

variable "user_ocid" {
  description = "OCID of the OCI user that Terraform will authenticate as"
  type        = string
  sensitive   = true
}

variable "api_fingerprint" {
  description = "Fingerprint of the OCI API signing key (Profile → API Keys)"
  type        = string
  sensitive   = true
}

variable "api_private_key_path" {
  description = "Local path to the OCI API private key PEM file (~/.oci/oci_api_key.pem)"
  type        = string
  default     = "~/.oci/oci_api_key.pem"
}

variable "region" {
  description = "OCI region identifier (e.g. ap-mumbai-1, us-ashburn-1, eu-frankfurt-1)"
  type        = string
}

# ── SSH access ────────────────────────────────────────────────────────────────

variable "ssh_public_key" {
  description = "Public SSH key to install on the VM (contents of ~/.ssh/id_rsa.pub or similar)"
  type        = string
  sensitive   = true
}

variable "ssh_allowed_cidr" {
  description = "CIDR that is allowed to SSH into the VM. Use your home IP/32 for security, 0.0.0.0/0 to allow all."
  type        = string
  default     = "0.0.0.0/0"
}

# ── Repo ──────────────────────────────────────────────────────────────────────

variable "repo_url" {
  description = "Git repo URL to clone on first boot (used in cloud-init)"
  type        = string
  default     = "https://github.com/ankitsriv89/system-design.git"
}

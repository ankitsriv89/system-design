# =============================================================================
# main.tf — Oracle Cloud Infrastructure (OCI) free-tier VM for system-design
#
# Provisions:
#   - VCN + public subnet + internet gateway + route table
#   - Security list: SSH (22), HTTP (80), HTTPS (443) ingress
#   - ARM64 Ampere A1 compute instance (4 OCPU / 24 GB RAM — always free)
#   - 200 GB block volume attached to the instance
#   - Object Storage bucket (for future use / artefacts)
#
# State: local (terraform.tfstate) by default.
# To use Terraform Cloud instead, uncomment the cloud {} block below and
# remove or comment out this comment block.
# =============================================================================

terraform {
  # ---------------------------------------------------------------------------
  # LOCAL STATE (default) — state file lives at terraform/terraform.tfstate.
  # Add terraform.tfstate* to .gitignore (already there) — never commit state.
  # ---------------------------------------------------------------------------

  # ---------------------------------------------------------------------------
  # OPTIONAL: Terraform Cloud remote state.
  # Uncomment this block if you want to store state at app.terraform.io.
  # Create a free workspace first, then set tfc_organization/tfc_workspace
  # in terraform.tfvars.
  # ---------------------------------------------------------------------------
  # cloud {
  #   organization = var.tfc_organization
  #   workspaces {
  #     name = var.tfc_workspace
  #   }
  # }

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 6.0"
    }
  }

  required_version = ">= 1.6"
}

# ── Provider ──────────────────────────────────────────────────────────────────
provider "oci" {
  tenancy_ocid     = var.tenancy_ocid
  user_ocid        = var.user_ocid
  fingerprint      = var.api_fingerprint
  private_key_path = var.api_private_key_path
  region           = var.region
}

# ── Availability domain (first AD in the region) ──────────────────────────────
data "oci_identity_availability_domains" "ads" {
  compartment_id = var.tenancy_ocid
}

locals {
  # Most OCI free-tier regions have a single AD; index 0 is always valid.
  availability_domain = data.oci_identity_availability_domains.ads.availability_domains[0].name
}

# ── Latest Ubuntu 22.04 minimal ARM64 image ───────────────────────────────────
data "oci_core_images" "ubuntu_arm" {
  compartment_id           = var.compartment_ocid
  operating_system         = "Canonical Ubuntu"
  operating_system_version = "22.04"
  shape                    = "VM.Standard.A1.Flex"
  sort_by                  = "TIMECREATED"
  sort_order               = "DESC"

  # Only minimal (non-GPU) images
  filter {
    name   = "display_name"
    values = [".*Minimal.*"]
    regex  = true
  }
}

# =============================================================================
# NETWORK
# =============================================================================

resource "oci_core_vcn" "main" {
  compartment_id = var.compartment_ocid
  display_name   = "system-design-vcn"
  cidr_blocks    = ["10.0.0.0/16"]
  dns_label      = "sysdesign"
}

resource "oci_core_internet_gateway" "igw" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.main.id
  display_name   = "system-design-igw"
  enabled        = true
}

resource "oci_core_route_table" "public" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.main.id
  display_name   = "system-design-public-rt"

  route_rules {
    destination       = "0.0.0.0/0"
    network_entity_id = oci_core_internet_gateway.igw.id
  }
}

resource "oci_core_security_list" "main" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.main.id
  display_name   = "system-design-security-list"

  # Allow all outbound traffic
  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
    stateless   = false
  }

  # SSH — restrict to your IP in production (set ssh_allowed_cidr variable)
  ingress_security_rules {
    protocol  = "6" # TCP
    source    = var.ssh_allowed_cidr
    stateless = false

    tcp_options {
      min = 22
      max = 22
    }
  }

  # HTTP
  ingress_security_rules {
    protocol  = "6"
    source    = "0.0.0.0/0"
    stateless = false

    tcp_options {
      min = 80
      max = 80
    }
  }

  # HTTPS
  ingress_security_rules {
    protocol  = "6"
    source    = "0.0.0.0/0"
    stateless = false

    tcp_options {
      min = 443
      max = 443
    }
  }

  # ICMP (ping) — useful for debugging
  ingress_security_rules {
    protocol  = "1" # ICMP
    source    = "0.0.0.0/0"
    stateless = false
  }
}

resource "oci_core_subnet" "public" {
  compartment_id    = var.compartment_ocid
  vcn_id            = oci_core_vcn.main.id
  display_name      = "system-design-public-subnet"
  cidr_block        = "10.0.1.0/24"
  dns_label         = "pub"
  route_table_id    = oci_core_route_table.public.id
  security_list_ids = [oci_core_security_list.main.id]

  # Public subnet — instances get a public IP by default
  prohibit_public_ip_on_vnic = false
}

# =============================================================================
# COMPUTE — Ampere A1 Flex (ARM64, always free: 4 OCPU / 24 GB)
# =============================================================================

resource "oci_core_instance" "vm" {
  compartment_id      = var.compartment_ocid
  availability_domain = local.availability_domain
  display_name        = "system-design-vm"
  shape               = "VM.Standard.A1.Flex"

  shape_config {
    # OCI free tier: up to 4 OCPUs and 24 GB total across all A1 instances.
    # Using all of it on one VM gives maximum headroom for 50 containers.
    ocpus         = 4
    memory_in_gbs = 24
  }

  source_details {
    source_type             = "image"
    source_id               = data.oci_core_images.ubuntu_arm.images[0].id
    boot_volume_size_in_gbs = 50 # Boot volume — OS + Docker images
  }

  create_vnic_details {
    subnet_id        = oci_core_subnet.public.id
    assign_public_ip = true
    hostname_label   = "system-design"
  }

  metadata = {
    # Public SSH key for the ubuntu user
    ssh_authorized_keys = var.ssh_public_key

    # cloud-init script: hardens SSH, installs git, pulls repo, prepares Docker group
    user_data = base64encode(templatefile("${path.module}/cloud-init.yaml", {
      repo_url = var.repo_url
    }))
  }

  # Preserve the instance if Terraform is accidentally run with destroy
  lifecycle {
    prevent_destroy = true
  }
}

# ── Reserve a static public IP ────────────────────────────────────────────────
# Attaches a reserved public IP to the instance's primary VNIC so the IP
# stays the same across instance stops/starts.
data "oci_core_vnic_attachments" "vm" {
  compartment_id = var.compartment_ocid
  instance_id    = oci_core_instance.vm.id
}

data "oci_core_vnic" "vm_primary" {
  vnic_id = data.oci_core_vnic_attachments.vm.vnic_attachments[0].vnic_id
}

resource "oci_core_public_ip" "static" {
  compartment_id = var.compartment_ocid
  lifetime       = "RESERVED"
  display_name   = "system-design-public-ip"
  private_ip_id  = data.oci_core_vnic.vm_primary.private_ip_id

  # Don't destroy the IP even if the instance is recreated — DNS continuity
  lifecycle {
    prevent_destroy = true
  }
}

# =============================================================================
# BLOCK VOLUME — 150 GB extra disk (OCI free tier: 200 GB total)
# =============================================================================

resource "oci_core_volume" "data" {
  compartment_id      = var.compartment_ocid
  availability_domain = local.availability_domain
  display_name        = "system-design-data"

  # 150 GB data volume + 50 GB boot = 200 GB total (free tier limit)
  size_in_gbs = 150
}

resource "oci_core_volume_attachment" "data" {
  attachment_type = "paravirtualized"
  instance_id     = oci_core_instance.vm.id
  volume_id       = oci_core_volume.data.id
  display_name    = "system-design-data-attach"

  # Hot-attach: instance does not need to stop
  is_pv_encryption_in_transit_enabled = false
}

# =============================================================================
# OBJECT STORAGE BUCKET
# =============================================================================

data "oci_objectstorage_namespace" "ns" {
  compartment_id = var.compartment_ocid
}

resource "oci_objectstorage_bucket" "artefacts" {
  compartment_id = var.compartment_ocid
  namespace      = data.oci_objectstorage_namespace.ns.namespace
  name           = "system-design-artefacts"
  access_type    = "NoPublicAccess"
  storage_tier   = "Standard"
}

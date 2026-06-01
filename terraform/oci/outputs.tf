# =============================================================================
# outputs.tf — Values printed after a successful terraform apply
# =============================================================================

output "vm_public_ip" {
  description = "Reserved public IP of the VM — use this in DNS and .env files"
  value       = oci_core_public_ip.static.ip_address
}

output "vm_instance_id" {
  description = "OCID of the compute instance"
  value       = oci_core_instance.vm.id
}

output "ssh_command" {
  description = "SSH command to connect to the VM"
  value       = "ssh -i ~/.ssh/<your-key> ubuntu@${oci_core_public_ip.static.ip_address}"
}

output "object_storage_bucket" {
  description = "Name of the artefacts Object Storage bucket"
  value       = oci_objectstorage_bucket.artefacts.name
}

output "block_volume_id" {
  description = "OCID of the attached data block volume"
  value       = oci_core_volume.data.id
}

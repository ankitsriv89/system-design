# Terraform — Infrastructure

Two independent Terraform roots, one per cloud provider.

```
terraform/
├── oci/    — Oracle Cloud free-tier ARM64 VM (permanent, $0/month)
└── aws/    — AWS Spot t4g.large ARM64 (temporary testing, ~$10/month)
```

Each directory is a self-contained Terraform root. Run `terraform init` inside the relevant subdirectory.

## OCI — Oracle Cloud (permanent host)

4 vCPU / 24 GB RAM Ampere A1 — always-free tier. Runs all 50 projects permanently.

```bash
cd terraform/oci
terraform init && terraform apply
```

See [oci/README.md](oci/README.md) for full details.

## AWS — Spot instance (cloud testing)

`t4g.large` ARM64 Spot in `ap-south-1` (Mumbai). Deploys all current projects automatically on first boot. Destroy when done — billed per second.

```bash
cd terraform/aws
cp terraform.tfvars.example terraform.tfvars  # fill in ssh_public_key
terraform init && terraform apply
# outputs: live Spot price, service URLs, SSH command, Grafana URL
terraform destroy  # when finished testing
```

See [aws/README.md](aws/README.md) for full details.

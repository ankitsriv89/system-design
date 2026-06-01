# Terraform — OCI Free Tier Infrastructure

Provisions the Oracle Cloud Infrastructure resources for all 50 system-design projects on a single always-free ARM64 VM.

## What it creates

| Resource | Details |
|---|---|
| VCN + subnet | 10.0.0.0/16, public subnet 10.0.1.0/24 |
| Internet Gateway + Route Table | Public internet access |
| Security List | Ingress: SSH (22), HTTP (80), HTTPS (443), ICMP |
| Compute Instance | VM.Standard.A1.Flex — 4 OCPU / 24 GB (ARM64, always free) |
| Reserved Public IP | Static IP — survives instance stops/starts |
| Block Volume | 150 GB data volume (50 GB boot + 150 GB = 200 GB total, free tier limit) |
| Object Storage Bucket | `system-design-artefacts` |

State is stored **locally** in `terraform.tfstate` (gitignored — never commit it).
To switch to Terraform Cloud remote state later, uncomment the `cloud {}` block in `main.tf`.

---

## One-time setup (do this before `terraform apply`)

### 1. Create OCI API credentials

In the OCI Console:
1. Profile icon (top-right) → **My profile** → **API keys** (left sidebar) → **Add API key**
2. Choose **Generate API key pair** — download the private key to `~/.oci/oci_api_key.pem`
3. OCI shows a **Configuration File Preview** after — copy it somewhere safe; it contains your `tenancy`, `user`, `fingerprint`, and `region`
4. `chmod 600 ~/.oci/oci_api_key.pem`

> **Note:** Your `compartment_ocid` = your `tenancy_ocid` for a personal/root-compartment setup. No need to create a sub-compartment.

### 2. Create your variables file

```bash
cd terraform/
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars — fill in all values from the OCI config snippet
```

Never commit `terraform.tfvars` — it is gitignored.

---

## Deploy

```bash
cd terraform/

# Download the OCI provider
terraform init

# Preview what will be created
terraform plan

# Create the infrastructure (takes ~2 minutes)
terraform apply
```

After apply completes, the VM's public IP is printed:

```
Outputs:
  vm_public_ip = "140.xxx.xxx.xxx"
  ssh_command  = "ssh -i ~/.ssh/<your-key> ubuntu@140.xxx.xxx.xxx"
```

---

## Bootstrap the VM

SSH in immediately after `terraform apply`:

```bash
ssh -i ~/.ssh/<your-key> ubuntu@<VM_PUBLIC_IP>

# cloud-init already cloned the repo — just run:
bash ~/system-design/infra/setup.sh
```

Then log out and back in (Docker group), and deploy your first project:

```bash
cd ~/system-design
./infra/deploy.sh 02-url-shortener
```

---

## Mount the data block volume (one-time, on the VM)

The 150 GB block volume is attached but not mounted. Do this once after the VM starts:

```bash
# Find the device (usually /dev/sdb or /dev/oracleoci/oraclevdb)
lsblk

# Format (only if new — this ERASES the volume)
sudo mkfs.ext4 /dev/sdb

# Mount
sudo mkdir -p /data
sudo mount /dev/sdb /data
sudo chown ubuntu:ubuntu /data

# Make permanent (survives reboots)
echo '/dev/sdb /data ext4 defaults,nofail 0 2' | sudo tee -a /etc/fstab
```

Use `/data` for Docker volumes, postgres data, and project artefacts.

---

## Update infra (after Terraform changes)

```bash
cd terraform/
terraform plan    # review changes
terraform apply   # apply
```

The compute instance has `lifecycle { prevent_destroy = true }` — Terraform will
refuse to destroy it even if you run `terraform destroy`. This prevents accidental
data loss. To override: remove the lifecycle block, then run destroy.

---

## Add a new project (Terraform is not involved)

Terraform only manages OCI resources. Adding a new project means:
1. Build the project in `projects/NN-name/` with a `docker-compose.yml`
2. Add the Caddy route to `infra/caddy/Caddyfile`
3. Push to GitHub
4. On the VM: `./infra/deploy.sh NN-name && sudo systemctl reload caddy`

See [DEPLOYMENT.md](../DEPLOYMENT.md) for the full workflow.

---

## Cost

**$0/month.** OCI Ampere A1 free tier:

| Resource | Free Allowance |
|---|---|
| Compute | 4 OCPUs + 24 GB RAM (A1 shape, no expiry) |
| Block Volume | 200 GB total |
| Object Storage | 20 GB |
| Outbound Data | 10 TB/month |

This config uses 4 OCPU / 24 GB / 200 GB — exactly the free allowance.

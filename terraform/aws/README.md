# AWS Spot Instance — System Design Testing

Spins up a `t4g.large` ARM64 Spot instance in `ap-south-1` (Mumbai) to test all system-design projects. Automatically deploys every project on first boot.

## Cost

| | $/hr | $/day | $/month |
|---|---|---|---|
| Spot (current) | ~$0.015 | ~$0.36 | ~$10.70 |
| On-demand cap | $0.0448 | $1.08 | $32.70 |

**Billed per-second. Run `terraform destroy` when done testing.**

## Prerequisites

- Terraform ≥ 1.5: `brew install terraform`
- AWS CLI configured: `aws configure` (profile needs EC2 + VPC + EIP permissions)
- SSH key pair available locally

## Quick Start

```bash
cd terraform/aws

# 1. Copy and fill in variables
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars

# 2. Init providers
terraform init

# 3. Preview what will be created
terraform plan

# 4. Create the instance (~2 min for Spot fulfillment + ~5 min cloud-init)
terraform apply

# 5. SSH in and check deployment status
ssh -i ~/.ssh/id_ed25519 ubuntu@<public_ip>
cat ~/deployment-status.txt
docker ps
```

## Outputs after apply

```
instance_summary = {
  instance_id   = "i-0abc..."
  instance_type = "t4g.large"
  region        = "ap-south-1"
  az            = "ap-south-1c"
  arch          = "arm64 (Graviton2)"
  vcpu          = 2
  ram_gb        = 8
  ami_id        = "ami-0..."
  ami_name      = "ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-arm64-server-..."
}

spot_pricing = {
  current_spot_price_usd_hr  = "0.0147"
  max_bid_price_usd_hr       = "0.0448"
  estimated_monthly_usd      = "~$10.73"
  spot_savings_vs_on_demand  = "~67%"
  interruption_behavior      = "stop (EBS root survives)"
}

access = {
  public_ip      = "13.x.x.x"
  ssh_command    = "ssh -i ~/.ssh/id_ed25519 ubuntu@13.x.x.x"
  grafana_url    = "http://13.x.x.x:3000"
  prometheus_url = "http://13.x.x.x:9090"
  services = {
    01-rate-limiter  = "http://13.x.x.x:8081"
    02-url-shortener = "http://13.x.x.x:8082"
    03-pastebin      = "http://13.x.x.x:8082"
    04-unique-id-gen = "http://13.x.x.x:8083"
  }
}

cost_estimate = {
  spot_price_hr    = "0.0147"
  daily_cost_usd   = "~$0.353"
  monthly_cost_usd = "~$10.73"
}
```

## Monitoring

Once cloud-init completes (~5–7 min):

- **Grafana**: `http://<ip>:3000` — login `admin` / `admin`
  - `Host System` dashboard: CPU, memory, disk, network, **uptime**
  - `Unique ID Generator` dashboard: generation rate, clock rollbacks, lease health
  - Per-project dashboards: rate-limiter, url-shortener, pastebin
- **Prometheus**: `http://<ip>:9090`

## Spot interruption handling

The instance is configured with `instance_interruption_behavior = "stop"`. If AWS reclaims the Spot, the instance stops (root EBS volume survives). Restart it from the AWS Console or via:

```bash
aws ec2 start-instances --instance-ids <instance_id> --region ap-south-1
```

## Destroy when done

```bash
terraform destroy
```

This terminates the instance, releases the EIP, and removes all resources. You will no longer be charged.

# =============================================================================
# terraform/aws/outputs.tf — What gets printed after terraform apply
# =============================================================================

locals {
  spot_price      = tonumber(data.aws_ec2_spot_price.current.spot_price)
  on_demand_price = 0.0448
}

output "instance_summary" {
  description = "Spot instance details"
  value = {
    instance_id   = aws_spot_instance_request.main.spot_instance_id
    instance_type = var.instance_type
    region        = var.aws_region
    az            = "${var.aws_region}${var.availability_zone_suffix}"
    arch          = "arm64 (Graviton2)"
    vcpu          = 2
    ram_gb        = 8
    ami_id        = data.aws_ami.ubuntu_arm64.id
    ami_name      = data.aws_ami.ubuntu_arm64.name
  }
}

output "spot_pricing" {
  description = "Spot pricing information"
  value = {
    current_spot_price_usd_hr = data.aws_ec2_spot_price.current.spot_price
    max_bid_price_usd_hr      = var.spot_price_max
    on_demand_price_usd_hr    = format("%.4f", local.on_demand_price)
    estimated_monthly_usd     = format("~$%.2f", local.spot_price * 730)
    on_demand_monthly_usd     = format("~$%.2f", local.on_demand_price * 730)
    spot_savings_vs_on_demand = format("~%.0f%%", (1 - local.spot_price / local.on_demand_price) * 100)
    interruption_behavior     = "stop (EBS root survives)"
  }
}

output "network" {
  description = "Network identifiers"
  value = {
    vpc_id    = aws_vpc.main.id
    subnet_id = aws_subnet.public.id
    sg_id     = aws_security_group.main.id
  }
}

output "access" {
  description = "How to reach the instance"
  value = {
    public_ip      = aws_eip.main.public_ip
    public_dns     = aws_eip.main.public_dns
    ssh_command    = "ssh -i ~/.ssh/id_ed25519 ubuntu@${aws_eip.main.public_ip}"
    grafana_url    = "http://${aws_eip.main.public_ip}:3000"
    prometheus_url = "http://${aws_eip.main.public_ip}:9090"
    services = {
      "01-rate-limiter"  = "http://${aws_eip.main.public_ip}:8081"
      "02-url-shortener" = "http://${aws_eip.main.public_ip}:8082"
      "03-pastebin"      = "http://${aws_eip.main.public_ip}:8082"
      "04-unique-id-gen" = "http://${aws_eip.main.public_ip}:8083"
    }
  }
}

output "cost_estimate" {
  description = "Running cost reminders — destroy when done testing"
  value = {
    note             = "Spot billed per-second. Run: terraform destroy"
    spot_price_hr    = data.aws_ec2_spot_price.current.spot_price
    daily_cost_usd   = format("~$%.3f", local.spot_price * 24)
    monthly_cost_usd = format("~$%.2f", local.spot_price * 730)
    eip_cost_note    = "EIP free while attached; $0.005/hr if instance stopped"
  }
}

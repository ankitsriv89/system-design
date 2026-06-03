# =============================================================================
# terraform/aws-multi/outputs.tf
# =============================================================================

locals {
  infra_spot   = tonumber(try(data.aws_ec2_spot_price.infra.spot_price, 0.008))
  go_spot      = tonumber(try(data.aws_ec2_spot_price.go.spot_price, 0.008))
  java_spot    = tonumber(try(data.aws_ec2_spot_price.java.spot_price, 0.015))
  other_spot   = tonumber(try(data.aws_ec2_spot_price.other.spot_price, 0.008))
  total_hr     = local.infra_spot + local.go_spot + local.java_spot + local.other_spot
}

data "aws_ec2_spot_price" "infra" {
  instance_type     = "t4g.medium"
  availability_zone = "${var.aws_region}${var.az_suffix}"
  filter { name = "product-description"; values = ["Linux/UNIX"] }
}
data "aws_ec2_spot_price" "go" {
  instance_type     = "t4g.medium"
  availability_zone = "${var.aws_region}${var.az_suffix}"
  filter { name = "product-description"; values = ["Linux/UNIX"] }
}
data "aws_ec2_spot_price" "java" {
  instance_type     = "t4g.large"
  availability_zone = "${var.aws_region}${var.az_suffix}"
  filter { name = "product-description"; values = ["Linux/UNIX"] }
}
data "aws_ec2_spot_price" "other" {
  instance_type     = "t4g.medium"
  availability_zone = "${var.aws_region}${var.az_suffix}"
  filter { name = "product-description"; values = ["Linux/UNIX"] }
}

output "access" {
  description = "SSH and service URLs for all instances"
  value = {
    infra = {
      public_ip      = aws_eip.infra.public_ip
      private_ip     = aws_spot_instance_request.infra.private_ip
      ssh            = "ssh -i ~/.ssh/id_ed25519 ubuntu@${aws_eip.infra.public_ip}"
      prometheus     = "http://${aws_eip.infra.public_ip}:9090"
      grafana        = "http://${aws_eip.infra.public_ip}:3000"
      postgres_note  = "reachable from runners at ${aws_spot_instance_request.infra.private_ip}:5432"
      kafka_note     = "reachable from runners at ${aws_spot_instance_request.infra.private_ip}:9092"
    }
    go_runner = {
      public_ip = aws_eip.go_runner.public_ip
      ssh       = "ssh -i ~/.ssh/id_ed25519 ubuntu@${aws_eip.go_runner.public_ip}"
      note      = "Go projects — ports 8081-8131 as assigned in infra/ports.md"
    }
    java_runner = {
      public_ip = aws_eip.java_runner.public_ip
      ssh       = "ssh -i ~/.ssh/id_ed25519 ubuntu@${aws_eip.java_runner.public_ip}"
      note      = "Java/Spring Boot projects — p10 on :8091, etc."
    }
    other_runner = {
      public_ip = aws_eip.other_runner.public_ip
      ssh       = "ssh -i ~/.ssh/id_ed25519 ubuntu@${aws_eip.other_runner.public_ip}"
      note      = "Python/Node projects"
    }
  }
}

output "cost_estimate" {
  description = "Spot cost summary — destroy when done testing"
  value = {
    infra_hr        = format("$%.4f", local.infra_spot)
    go_runner_hr    = format("$%.4f", local.go_spot)
    java_runner_hr  = format("$%.4f", local.java_spot)
    other_runner_hr = format("$%.4f", local.other_spot)
    total_hr        = format("~$%.3f", local.total_hr)
    total_day       = format("~$%.2f", local.total_hr * 24)
    note            = "EIPs free while attached. Run: terraform destroy when done."
  }
}

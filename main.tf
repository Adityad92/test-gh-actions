# dummy-terraform/main.tf
terraform {
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

resource "null_resource" "example" {
  provisioner "local-exec" {
    command = "echo Hello, Terraform!, and its value is ${var.dummy_variable}"
  }
}
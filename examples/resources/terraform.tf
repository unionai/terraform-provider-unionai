terraform {
  required_providers {
    unionai = {
      source  = "unionai/unionai"
      version = "0.1.0"
    }
  }
}

provider "unionai" {
  allowed_orgs = [
    "tryv2",
  ]
}

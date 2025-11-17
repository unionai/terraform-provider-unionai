terraform {
  required_providers {
    unionai = {
      source  = "unionai/unionai"
      version = "0.1.0"
    }
  }
}

provider "unionai" {
  api_key = "<your-api-key-goes-here>"
}

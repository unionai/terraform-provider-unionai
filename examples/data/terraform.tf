terraform {
  required_providers {
    unionai = {
      source  = "unionai/unionai"
      version = "0.1.0"
    }
  }
}

# You can also specify environment variables UNIONAI_API_KEY
provider "unionai" {
  api_key = "<your-api-key-goes-here>"
}

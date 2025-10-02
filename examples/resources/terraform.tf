terraform {
  required_providers {
    unionai = {
      source  = "unionai/enterprise"
      version = "0.1.0"
    }
  }
}

# You can also specify environment variables UNIONAI_HOST, UNIONAI_CLIENT_ID, UNIONAI_CLIENT_SECRET
provider "unionai" {
  host          = "<your-host-name-goes-here>"
  client_id     = "<your-client-id-goes-here>"
  client_secret = "<your-client-secret-goes-here>"
}

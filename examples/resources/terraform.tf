terraform {
  required_providers {
    unionai = {
      source = "unionai/unionai"
    }
    # Only for the Karpenter example in node_groups.tf.
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
}

provider "unionai" {}

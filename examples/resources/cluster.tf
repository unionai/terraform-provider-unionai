resource "unionai_cluster" "example" {
  name = "prod-us-east-2"
}

resource "unionai_cluster" "gpu" {
  name              = "gpu-us-west-2"
  cluster_pool_name = "gpu-pool"
}

output "cluster_example" {
  value = unionai_cluster.example
}

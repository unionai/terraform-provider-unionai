resource "unionai_node_groups" "prod" {
  cluster = "prod-us-east-2"

  node_group {
    name                = "cpu-general"
    min_nodes           = 0
    max_nodes           = 10
    node_disk_size_gbs  = 100
    node_instance_types = ["m6i.2xlarge"]
    capacity_types      = ["ON_DEMAND"]
    labels              = { workload = "general" }
  }

  node_group {
    name                = "gpu-a100"
    min_nodes           = 0
    max_nodes           = 4
    node_disk_size_gbs  = 500
    node_instance_types = ["p4d.24xlarge"]
    capacity_types      = ["SPOT", "ON_DEMAND"]

    taint {
      key    = "nvidia.com/gpu"
      value  = "present"
      effect = "NoSchedule"
    }

    accelerator {
      device       = "A100"
      manufacturer = "NVIDIA"
      count        = 8
    }
  }
}

output "node_groups_prod" {
  value = unionai_node_groups.prod
}

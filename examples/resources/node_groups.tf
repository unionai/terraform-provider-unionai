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

# Node groups derived from Karpenter. Union's node groups describe the capacity Karpenter can
# launch, so read them off the EC2NodeClass and NodePools instead of repeating the values. Needs
# the kubernetes provider configured for the EKS cluster.
locals {
  karpenter_node_pools = {
    general = {
      weight             = 10
      instance_types     = ["m7i.2xlarge", "m7i.4xlarge"]
      capacity_types     = ["spot", "on-demand"]
      limits             = { cpu = "1000", memory = "4000Gi" }
      labels             = { workload = "general" }
      taints             = []
      extra_requirements = []
    }
    gpu = {
      weight         = 50
      instance_types = ["g5.2xlarge", "g5.4xlarge"]
      capacity_types = ["on-demand"]
      limits         = { cpu = "256", memory = "1024Gi", "nvidia.com/gpu" = "16" }
      labels         = { workload = "gpu" }
      taints         = [{ key = "nvidia.com/gpu", value = "present", effect = "NoSchedule" }]
      extra_requirements = [
        { key = "karpenter.k8s.aws/instance-gpu-name", operator = "In", values = ["a10g"] },
        { key = "karpenter.k8s.aws/instance-gpu-manufacturer", operator = "In", values = ["nvidia"] },
        { key = "karpenter.k8s.aws/instance-gpu-count", operator = "In", values = ["1"] },
      ]
    }
  }
}

resource "kubernetes_manifest" "karpenter_node_class" {
  manifest = {
    apiVersion = "karpenter.k8s.aws/v1"
    kind       = "EC2NodeClass"
    metadata   = { name = "default" }
    spec = {
      role                       = "KarpenterNodeRole-prod-us-east-2"
      amiSelectorTerms           = [{ alias = "al2023@latest" }]
      subnetSelectorTerms        = [{ tags = { "karpenter.sh/discovery" = "prod-us-east-2" } }]
      securityGroupSelectorTerms = [{ tags = { "karpenter.sh/discovery" = "prod-us-east-2" } }]
      kubelet                    = { maxPods = 110 }
      blockDeviceMappings = [{
        deviceName = "/dev/xvda"
        ebs        = { volumeSize = "200Gi", volumeType = "gp3" }
      }]
    }
  }
}

resource "kubernetes_manifest" "karpenter_node_pool" {
  for_each = local.karpenter_node_pools

  manifest = {
    apiVersion = "karpenter.sh/v1"
    kind       = "NodePool"
    metadata   = { name = each.key }
    spec = {
      weight = each.value.weight
      limits = each.value.limits
      template = {
        metadata = { labels = each.value.labels }
        spec = {
          nodeClassRef = {
            group = "karpenter.k8s.aws"
            kind  = "EC2NodeClass"
            name  = kubernetes_manifest.karpenter_node_class.manifest.metadata.name
          }
          taints = each.value.taints
          requirements = concat([
            { key = "node.kubernetes.io/instance-type", operator = "In", values = each.value.instance_types },
            { key = "karpenter.sh/capacity-type", operator = "In", values = each.value.capacity_types },
            { key = "kubernetes.io/arch", operator = "In", values = ["amd64"] },
          ], each.value.extra_requirements)
        }
      }
    }
  }
}

locals {
  karpenter_node_class = kubernetes_manifest.karpenter_node_class.manifest.spec

  # Each NodePool's spec, with its requirements keyed by label for easy lookup.
  karpenter_node_groups = {
    for name, pool in kubernetes_manifest.karpenter_node_pool : name => {
      spec         = pool.manifest.spec
      requirements = { for requirement in pool.manifest.spec.template.spec.requirements : requirement.key => requirement.values }
    }
  }
}

resource "unionai_node_groups" "karpenter" {
  cluster = "prod-us-east-2"

  dynamic "node_group" {
    for_each = local.karpenter_node_groups

    content {
      name                = node_group.key
      node_instance_types = node_group.value.requirements["node.kubernetes.io/instance-type"]
      # Karpenter spells these "on-demand" and "amd64"; Union uses "ON_DEMAND" and "AMD64".
      capacity_types = [for capacity_type in node_group.value.requirements["karpenter.sh/capacity-type"] : upper(replace(capacity_type, "-", "_"))]
      architecture   = upper(one(node_group.value.requirements["kubernetes.io/arch"]))
      # A NodePool is bounded by limits, not a node count, so min_nodes and max_nodes stay unset.
      weight             = node_group.value.spec.weight
      limits             = node_group.value.spec.limits
      labels             = node_group.value.spec.template.metadata.labels
      max_pods           = local.karpenter_node_class.kubelet.maxPods
      node_disk_size_gbs = tonumber(trimsuffix(local.karpenter_node_class.blockDeviceMappings[0].ebs.volumeSize, "Gi"))

      dynamic "taint" {
        for_each = node_group.value.spec.template.spec.taints
        content {
          key    = taint.value.key
          value  = taint.value.value
          effect = taint.value.effect
        }
      }

      dynamic "accelerator" {
        for_each = contains(keys(node_group.value.requirements), "karpenter.k8s.aws/instance-gpu-name") ? [node_group.value.requirements] : []
        content {
          device       = upper(one(accelerator.value["karpenter.k8s.aws/instance-gpu-name"]))
          manufacturer = upper(one(accelerator.value["karpenter.k8s.aws/instance-gpu-manufacturer"]))
          count        = tonumber(one(accelerator.value["karpenter.k8s.aws/instance-gpu-count"]))
        }
      }
    }
  }
}

output "node_groups_prod" {
  value = unionai_node_groups.prod
}

data "unionai_node_groups" "example" {
  cluster = "prod-us-east-2"
}

output "node_groups_prod" {
  value = data.unionai_node_groups.example
}

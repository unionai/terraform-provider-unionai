data "unionai_dataplane" "example" {
  id = "union-us-east-2"
}

output "dataplane" {
  value = data.unionai_dataplane.example
}

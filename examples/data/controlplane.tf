data "unionai_controlplane" "example" {}

output "controlplane" {
  value = data.unionai_controlplane.example
}

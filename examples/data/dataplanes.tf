data "unionai_dataplanes" "dataplanes" {}

output "dataplanes" {
  value = data.unionai_dataplanes.dataplanes
}

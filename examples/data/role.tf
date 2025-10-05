data "unionai_role" "my_role" {
  id = "viewer"
}

output "my_role" {
  value = data.unionai_role.my_role
}

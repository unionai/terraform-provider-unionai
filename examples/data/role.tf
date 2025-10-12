data "unionai_role" "my_role" {
  id = "contributor"
}

output "role" {
  value = data.unionai_role.my_role
}

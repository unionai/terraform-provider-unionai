data "unionai_role" "my_role" {
  id = "contributor"
}

output "my_role" {
  value = data.unionai_role.my_role
}

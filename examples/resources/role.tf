resource "unionai_role" "example" {
  name        = "my-role-22"
  description = "Some test role"
  actions = [
    #"administer_account",
    "administer_project",
    "view_flyte_executions",
  ]
}

output "role_admin" {
  value = unionai_role.example
}

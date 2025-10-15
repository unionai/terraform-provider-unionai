resource "unionai_role" "example" {
  name        = "my-role-jan-71"
  description = "Some test role"
  actions = [
    "administer_account",
    #"administer_project",
  ]
}

output "role_admin" {
  value = unionai_role.example
}

resource "unionai_role" "admin" {
  name = "admin"
  actions = [
    "administer_account",
    "administer_project",
  ]
}

output "role_admin" {
  value = unionai_role.admin
}

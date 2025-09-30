resource "unionai_role" "admin" {
  name = "admin"
  actions = [
    "administer_account",
    "administer_project",
  ]
}

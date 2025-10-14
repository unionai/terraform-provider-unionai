data "unionai_user_access" "example" {
  user_id   = data.unionai_user.example.id
  policy_id = "admin"
}

output "user_access" {
  value = data.unionai_user_access.example
}

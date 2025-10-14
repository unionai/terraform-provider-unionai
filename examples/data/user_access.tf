data "unionai_user_access" "example" {
  user_id   = "nelson@union.ai"
  policy_id = "admin"
}

output "user_access" {
  value = data.unionai_user_access.example
}

data "unionai_policy_binding" "example" {
  user_id   = "nelson@union.ai"
  policy_id = "admin"
}

output "policy_binding" {
  value = data.unionai_policy_binding.example
}

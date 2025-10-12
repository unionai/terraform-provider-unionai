data "unionai_policy" "test_policy" {
  id = "viewer"
}

output "policy" {
  value = data.unionai_policy.test_policy
}

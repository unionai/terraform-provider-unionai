data "unionai_policy" "test_policy" {
  id = "viewer"
}

output "test_policy" {
  value = data.unionai_policy.test_policy
}

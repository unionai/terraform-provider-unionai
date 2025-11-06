resource "unionai_user_access" "nelson" {
  user   = unionai_user.nelson.id
  policy = data.unionai_policy.viewer.id
}

output "user_access" {
  value = unionai_user_access.nelson
}

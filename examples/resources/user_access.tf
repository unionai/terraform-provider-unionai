resource "unionai_user_access" "nelson" {
  user   = unionai_user.nelson.id
  policy = unionai_policy.project1_admins.id
}

output "user_access_nelson" {
  value = unionai_user_access.nelson
}

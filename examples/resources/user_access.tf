data "unionai_policy" "test" {
  id = "viewer"
}

resource "unionai_user_access" "nelson" {
  user   = unionai_user.nelson.id
  policy = data.unionai_policy.test.id
}

output "user_access" {
  value = unionai_user_access.nelson
}

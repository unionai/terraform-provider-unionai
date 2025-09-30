resource "unionai_policy_binding" "nelson" {
  user   = unionai_user.nelson.id
  policy = unionai_policy.project1_admins.id
}

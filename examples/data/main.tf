data "unionai_user" "nelson" {
  email = "nelson@union.ai"
}

data "unionai_policy" "contributor" {
  name = "contributor"
}

resource "unionai_policy_binding" "nelson" {
  user   = data.unionai_user.nelson.id
  policy = data.unionai_policy.contributor.id
}

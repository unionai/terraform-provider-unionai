locals {
  users = csvdecode(file("users.csv"))
}

resource "unionai_user" "nelson" {
  for_each   = { for user in local.users : user.email => user }
  first_name = each.value.first_name
  last_name  = each.value.last_name
  email      = each.value.email
}

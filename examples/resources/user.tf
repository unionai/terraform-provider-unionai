resource "unionai_user" "nelson" {
  first_name = "Nelson"
  last_name  = "Araujo"
  email      = "nelson+terraform-test@union.ai"
}

resource "unionai_user" "laura" {
  first_name = "Laura"
  last_name  = "Barton"
  email      = "laura@union.ai"
}

output "user_nelson" {
  value = unionai_user.nelson
}

output "user_laura" {
  value = unionai_user.laura
}

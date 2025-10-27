resource "unionai_user" "nelson" {
  first_name = "Nelson"
  last_name  = "Araujo"
  email      = "nelson+terraform-test@union.ai"
}

output "user_nelson" {
  value = unionai_user.nelson
}

resource "unionai_user" "nelson" {
  first_name = "Nelson"
  last_name  = "Araujo"
  email      = "nelson@union.ai"
}

output "user_nelson" {
  value = unionai_user.nelson
}

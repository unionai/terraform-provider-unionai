data "unionai_user" "example" {
  email = "nelson@union.ai"
}

output "user" {
  value = data.unionai_user.example
}

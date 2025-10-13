resource "unionai_api_key" "example" {
  id = "my-key"
}

output "api_key_id" {
  value = unionai_api_key.example.id
}

output "api_key" {
  sensitive = true
  value     = unionai_api_key.example.secret
}

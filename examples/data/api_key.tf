data "unionai_api_key" "example" {
  id = "dummy-api-key"
}

output "api_key" {
  value = data.unionai_api_key.example
}

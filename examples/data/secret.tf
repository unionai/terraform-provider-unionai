data "unionai_secret" "eager_key" {
  name = "EAGER_API_KEY"
}

output "secret_eager" {
  value = data.unionai_secret.eager_key
}

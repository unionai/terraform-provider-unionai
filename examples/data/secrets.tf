data "unionai_secrets" "all_secrets" {
}

output "all_secrets" {
	value = data.unionai_secrets.all_secrets.secrets
}

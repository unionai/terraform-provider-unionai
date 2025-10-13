data "unionai_application" "myapp" {
  id = "uctl-tryv2"
}

output "application" {
  value = data.unionai_application.myapp
}

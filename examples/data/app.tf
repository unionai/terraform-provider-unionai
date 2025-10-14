data "unionai_application" "myapp" {
  id = "tryv2-uctl"
}

output "application" {
  value = data.unionai_application.myapp
}

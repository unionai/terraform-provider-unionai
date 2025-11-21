resource "unionai_task_environment" "env" {
  id      = "env"
  path    = "./v2_task/hello.py"
  project = "nelson"
  domain  = "development"
}

output "task_environment" {
  value = unionai_task_environment.env
}

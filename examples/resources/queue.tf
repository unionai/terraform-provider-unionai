resource "unionai_queue" "example" {
  name     = "batch"
  clusters = ["*"]

  run_concurrency    = 10
  action_concurrency = 100
  depth              = 1000

  priority = "medium"
  fairness = "round_robin"

  max_resources = {
    cpu               = "16"
    memory            = "64Gi"
    ephemeral_storage = "1Ti"
  }
  scheduling = "strict_fifo"
}

output "queue_batch" {
  value = unionai_queue.example
}

data "unionai_queue" "jantest" {
  id = "jantest"
}

resource "unionai_queue" "batch" {
  name     = "jantest1"
  clusters = ["aws-prod-use2"]
  priority = "min"
  depth    = 21
  max_resources = {
    cpu    = "16"
    memory = "64Gi"
    # ephemeral_storage = "1Ti"
  }
  #drain    = true
  scheduling = "strict_fifo"
}

data "unionai_queue" "test123" {
  id = "jantest"
}

output "jantestqueue" {
  value = data.unionai_queue.test123.scheduling
}
output "jantestqueue_rsources" {
  value = data.unionai_queue.test123.max_resources
}

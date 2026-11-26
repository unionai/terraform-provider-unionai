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


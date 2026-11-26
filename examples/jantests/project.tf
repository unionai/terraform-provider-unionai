resource "unionai_project_domain_attributes" "test" {
  project = "jan-terraform-provider-tests"
  domain  = "development"

  attributes = {
    projectQuotaCpu       = "2"
    projectQuotaMemory    = "2Gi"
    projectQuotaNvidiaGpu = "1"
  }
}

resource "unionai_project_domain_attributes" "test1" {
  project = "jan-terraform-provider-tests"
  domain  = "staging"

  attributes = {
    projectQuotaCpu       = "2"
    projectQuotaMemory    = "2Gi"
    projectQuotaNvidiaGpu = "1"
  }
}


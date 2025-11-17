---
page_title: "unionai_controlplane Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about the Union.ai controlplane.
---

# unionai_controlplane (Data Source)

Retrieves information about the Union.ai controlplane. The controlplane manages and orchestrates the dataplanes.

## Example Usage

```terraform
data "unionai_controlplane" "main" {}

output "controlplane_info" {
  value = data.unionai_controlplane.main
}
```

## Schema

### Read-Only

The schema for this data source will return all available attributes of the controlplane configuration.

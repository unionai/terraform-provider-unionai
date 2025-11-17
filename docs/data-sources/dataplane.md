---
page_title: "unionai_dataplane Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves information about a Union.ai dataplane.
---

# unionai_dataplane (Data Source)

Retrieves information about a Union.ai dataplane. Dataplanes are compute resources where workflows are executed.

## Example Usage

```terraform
data "unionai_dataplane" "production" {
  id = "dataplane-id"
}

output "dataplane_status" {
  value = data.unionai_dataplane.production.status
}
```

## Schema

### Required

- `id` (String) The unique identifier of the dataplane.

### Read-Only

The schema for this data source will return all available attributes of the dataplane as configured in your Union.ai organization.

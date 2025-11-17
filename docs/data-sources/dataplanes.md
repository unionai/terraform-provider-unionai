---
page_title: "unionai_dataplanes Data Source - terraform-provider-unionai"
subcategory: ""
description: |-
  Retrieves a list of Union.ai dataplanes.
---

# unionai_dataplanes (Data Source)

Retrieves a list of Union.ai dataplanes. This data source can be used to discover all dataplanes available in your organization.

## Example Usage

```terraform
data "unionai_dataplanes" "all" {}

output "dataplane_count" {
  value = length(data.unionai_dataplanes.all.dataplanes)
}
```

## Schema

### Read-Only

- `dataplanes` (List of Object) List of all dataplanes in the organization.

The schema for this data source will return all available dataplanes with their respective attributes.

# Examples
## How to run these examples in dev mode
When testing changes on the terraform provider you want to point to a local version of the provider instead of getting it from the offical terraform registry
- Run `go build` in the root directory of this repo -> creates `terraform-provider-enterprise`
- Create a `.terraformrc` file in the examples dir or any directory:
``` terraform
provider_installation {
  dev_overrides {
    "unionai/enterprise" = "/Users/janfiedler/Documents/Union/repos/unionai-terraform-provider" # Path to this repo
  }
  direct {}
}
```
- Set the `TF_CLI_CONFIG_FILE` env var to this file
``` bash
export TF_CLI_CONFIG_FILE=/Users/janfiedler/Documents/Union/repos/unionai-terraform-provider/examples/.terraformrc
```
- By unsetting this env var you can switch to getting it from the official registry again ;)
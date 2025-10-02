# Union Terraform Provider

## Provider

Configure the provider by setting the following parameters:

    provider "unionai" {
      host          = "<your-host-name-goes-here>"
      client_id     = "<your-client-id-goes-here>"
      client_secret = "<your-client-secret-goes-here>"
    }

You can also use the environment variables `UNIONAI_HOST`, `UNIONAI_CLIENT_ID`,
and `UNIONAI_CLIENT_SECRET` to provide the values above.


## Credentials

As Terraform runs unattended and will not have access to the browser, you need a
non-Web credential to provide to the provider.

You can use `uctl` to accomplish that. Ensure you have these settings when
creating your application:

    grantTypes:
      - CLIENT_CREDENTIALS

    responseTypes:
      - TOKEN

1. Create an application definition file, e.g., `myapp.yaml`

    clientId: <your-client-id>
    clientName: <your-client-name>
    clientUri: https://dummyclienturi
    grantTypes:
      - AUTHORIZATION_CODE
      - CLIENT_CREDENTIALS
      - IMPLICIT
    logoUri: https://logouri
    policyUri: https://dummypolicyuri
    redirectUris:
      - https://dummy/callback
    responseTypes:
      - CODE
      - TOKEN
    tokenEndpointAuthMethod: CLIENT_SECRET_POST
    tosUri: https://dummytosuri

   > All the fields that has `dummy` in its value are not used by the non-Web
   > applications, yet they are required fields by the OIDC provider. It is up
   > to you to have valid URLs or not. One good use for real URLs would be the
   > case you want someone looking at the app details, e.g. with `uctl get
   > oauth-app -o yaml`.

2. Invoke `uctl` to create the client ID and secret:

       uctl create oauth-app --appSpec myapp.yaml

3. Input the `host`, `client_id`, and `client_secret` into the provider section.


## Developer Setup

0.  Have a shared folder for repos

        $ mkdir src

1.  Clone this repository

        $ git clone git@github.com:unionai/unionai-terraform-provider

2.  Clone the `cloud` repo as a sibling directory

        $ git clone git@github.com:unionai/cloud


## Building

    $ cd unionai-terraform-provider
    $ go build


## Test the provider

When testing changes on the terraform provider you want to point to a local
version of the provider instead of getting it from the offical terraform
registry

- Run `go build` in the root directory of this repo -> creates
  `terraform-provider-enterprise`

- Create a `.terraformrc` file in the examples dir or any directory:

    provider_installation {
      dev_overrides {
        "unionai/enterprise" = "<path-to-the-binary-built>"
      }
      direct {}
    }

- Set the `TF_CLI_CONFIG_FILE` env var to this file

    export TF_CLI_CONFIG_FILE="<path-to-the-terraformrc>"/.terraformrc

- By unsetting this env var you can switch to getting it from the official
  registry again ;)


package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/flyteorg/flyte/v2/gen/go/flyteidl2/secret"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SecretResource{}
var _ resource.ResourceWithImportState = &SecretResource{}

func NewSecretResource() resource.Resource {
	return &SecretResource{}
}

// SecretResource defines the resource implementation.
type SecretResource struct {
	conn secret.SecretServiceClient
	org  string
}

// SecretResourceModel describes the resource data model.
type SecretResourceModel struct {
	Id           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Organization types.String `tfsdk:"organization"`
	Domain       types.String `tfsdk:"domain"`
	Project      types.String `tfsdk:"project"`
	Value        types.String `tfsdk:"value"`
	BinaryValue  types.String `tfsdk:"binary_value"`
	Type         types.String `tfsdk:"type"`
}

func (r *SecretResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *SecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Secret resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Secret identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Secret name",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization scope for the secret",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Domain scope for the secret",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project scope for the secret",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "String value of the secret (mutually exclusive with binary_value)",
			},
			"binary_value": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64-encoded binary value of the secret (mutually exclusive with value)",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Secret type (generic or image_pull_secret). Defaults to generic",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.conn = secret.NewSecretServiceClient(client.conn)
	if r.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *secret.SecretServiceClient, got: %T. Please report this issue to the provider developers.", r.conn),
		)
		return
	}
	r.org = client.org
}

func (r *SecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that exactly one of value or binary_value is set
	hasValue := !data.Value.IsNull() && !data.Value.IsUnknown()
	hasBinaryValue := !data.BinaryValue.IsNull() && !data.BinaryValue.IsUnknown()

	if !hasValue && !hasBinaryValue {
		resp.Diagnostics.AddError("Missing secret value", "Either 'value' or 'binary_value' must be specified")
		return
	}

	if hasValue && hasBinaryValue {
		resp.Diagnostics.AddError("Conflicting secret values", "Only one of 'value' or 'binary_value' can be specified")
		return
	}

	data.Id = data.Name

	// Set organization from provider if not specified
	if data.Organization.IsNull() || data.Organization.IsUnknown() {
		data.Organization = types.StringValue(r.org)
	}

	// Check if secret already exists
	if _, err := r.conn.GetSecret(ctx, &secret.GetSecretRequest{
		Id: &secret.SecretIdentifier{
			Name:         data.Name.ValueString(),
			Organization: data.Organization.ValueString(),
			Domain:       data.Domain.ValueString(),
			Project:      data.Project.ValueString(),
		},
	}); err == nil {
		resp.Diagnostics.AddError("Secret already exists", fmt.Sprintf("Secret %s already exists", data.Name.ValueString()))
		return
	}

	// Parse secret type
	secretType := secret.SecretType_SECRET_TYPE_GENERIC
	if !data.Type.IsNull() && !data.Type.IsUnknown() {
		typeStr := strings.ToUpper(data.Type.ValueString())
		if typeStr == "IMAGE_PULL_SECRET" {
			secretType = secret.SecretType_SECRET_TYPE_IMAGE_PULL_SECRET
		} else if typeStr != "GENERIC" && typeStr != "" {
			resp.Diagnostics.AddError("Invalid secret type", fmt.Sprintf("Secret type must be 'generic' or 'image_pull_secret', got: %s", data.Type.ValueString()))
			return
		}
	}
	data.Type = types.StringValue(strings.ToLower(strings.TrimPrefix(secret.SecretType_name[int32(secretType)], "SECRET_TYPE_")))

	// Build SecretSpec
	secretSpec := &secret.SecretSpec{
		Type: secretType,
	}

	if hasValue {
		secretSpec.Value = &secret.SecretSpec_StringValue{
			StringValue: data.Value.ValueString(),
		}
	} else {
		// Decode base64 binary value
		binaryData, err := base64.StdEncoding.DecodeString(data.BinaryValue.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid binary_value", fmt.Sprintf("binary_value must be valid base64: %s", err))
			return
		}
		secretSpec.Value = &secret.SecretSpec_BinaryValue{
			BinaryValue: binaryData,
		}
	}

	createRequest := &secret.CreateSecretRequest{
		Id: &secret.SecretIdentifier{
			Name:         data.Name.ValueString(),
			Organization: data.Organization.ValueString(),
			Domain:       data.Domain.ValueString(),
			Project:      data.Project.ValueString(),
		},
		SecretSpec: secretSpec,
	}

	tflog.Debug(ctx, "CreateSecret request", map[string]interface{}{
		"secret(create)": createRequest,
	})

	_, err := r.conn.CreateSecret(ctx, createRequest)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create secret, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	secretResp, err := r.conn.GetSecret(ctx, &secret.GetSecretRequest{
		Id: &secret.SecretIdentifier{
			Name:         data.Name.ValueString(),
			Organization: data.Organization.ValueString(),
			Domain:       data.Domain.ValueString(),
			Project:      data.Project.ValueString(),
		},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read secret, got error: %s", err))
		return
	}

	data.Id = types.StringValue(secretResp.Secret.Id.Name)
	data.Name = types.StringValue(secretResp.Secret.Id.Name)
	data.Organization = types.StringValue(secretResp.Secret.Id.Organization)

	if secretResp.Secret.Id.Domain != "" {
		data.Domain = types.StringValue(secretResp.Secret.Id.Domain)
	}
	if secretResp.Secret.Id.Project != "" {
		data.Project = types.StringValue(secretResp.Secret.Id.Project)
	}

	if secretResp.Secret.SecretMetadata != nil {
		secretType := strings.ToLower(strings.TrimPrefix(secret.SecretType_name[int32(secretResp.Secret.SecretMetadata.Type)], "SECRET_TYPE_"))
		data.Type = types.StringValue(secretType)
	}

	// Note: The API doesn't return the secret value for security reasons,
	// so we keep the values from state

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SecretResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that exactly one of value or binary_value is set
	hasValue := !data.Value.IsNull() && !data.Value.IsUnknown()
	hasBinaryValue := !data.BinaryValue.IsNull() && !data.BinaryValue.IsUnknown()

	if !hasValue && !hasBinaryValue {
		resp.Diagnostics.AddError("Missing secret value", "Either 'value' or 'binary_value' must be specified")
		return
	}

	if hasValue && hasBinaryValue {
		resp.Diagnostics.AddError("Conflicting secret values", "Only one of 'value' or 'binary_value' can be specified")
		return
	}

	// Parse secret type
	secretType := secret.SecretType_SECRET_TYPE_GENERIC
	if !data.Type.IsNull() && !data.Type.IsUnknown() {
		typeStr := strings.ToUpper(data.Type.ValueString())
		if typeStr == "IMAGE_PULL_SECRET" {
			secretType = secret.SecretType_SECRET_TYPE_IMAGE_PULL_SECRET
		} else if typeStr != "GENERIC" && typeStr != "" {
			resp.Diagnostics.AddError("Invalid secret type", fmt.Sprintf("Secret type must be 'generic' or 'image_pull_secret', got: %s", data.Type.ValueString()))
			return
		}
	}

	// Build SecretSpec
	secretSpec := &secret.SecretSpec{
		Type: secretType,
	}

	if hasValue {
		secretSpec.Value = &secret.SecretSpec_StringValue{
			StringValue: data.Value.ValueString(),
		}
	} else {
		// Decode base64 binary value
		binaryData, err := base64.StdEncoding.DecodeString(data.BinaryValue.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid binary_value", fmt.Sprintf("binary_value must be valid base64: %s", err))
			return
		}
		secretSpec.Value = &secret.SecretSpec_BinaryValue{
			BinaryValue: binaryData,
		}
	}

	updateRequest := &secret.UpdateSecretRequest{
		Id: &secret.SecretIdentifier{
			Name:         data.Name.ValueString(),
			Organization: data.Organization.ValueString(),
			Domain:       data.Domain.ValueString(),
			Project:      data.Project.ValueString(),
		},
		SecretSpec: secretSpec,
	}

	tflog.Debug(ctx, "UpdateSecret request", map[string]interface{}{
		"secret(update)": updateRequest,
	})

	_, err := r.conn.UpdateSecret(ctx, updateRequest)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update secret, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SecretResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.conn.DeleteSecret(ctx, &secret.DeleteSecretRequest{
		Id: &secret.SecretIdentifier{
			Name:         data.Name.ValueString(),
			Organization: data.Organization.ValueString(),
			Domain:       data.Domain.ValueString(),
			Project:      data.Project.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete secret, got error: %s", err))
		return
	}
}

func (r *SecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

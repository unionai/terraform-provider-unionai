package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/unionai/cloud/gen/pb-go/common"
	"github.com/unionai/cloud/gen/pb-go/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

// UserDataSource defines the data source implementation.
type UserDataSource struct {
	conn identity.UserServiceClient
	org  string
}

// UserDataSourceModel describes the data source data model.
type UserDataSourceModel struct {
	Id        types.String `tfsdk:"id"`
	FirstName types.String `tfsdk:"first_name"`
	LastName  types.String `tfsdk:"last_name"`
	Email     types.String `tfsdk:"email"`
}

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "User data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "User identifier",
				Optional:            true,
			},
			"first_name": schema.StringAttribute{
				MarkdownDescription: "First name of the user",
				Computed:            true,
			},
			"last_name": schema.StringAttribute{
				MarkdownDescription: "Last name of the user",
				Computed:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Email address of the user",
				Optional:            true,
			},
		},
	}
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*providerContext)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *providerContext, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.conn = identity.NewUserServiceClient(client.conn)
	if d.conn == nil {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *identity.UserServiceClient, got: %T. Please report this issue to the provider developers.", d.conn),
		)
		return
	}
	d.org = client.org
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var user *common.User

	if !data.Email.IsUnknown() {
		users, err := d.conn.ListUsers(ctx, &identity.ListUsersRequest{
			Organization: d.org,
			Request: &common.ListRequest{
				Filters: []*common.Filter{
					{
						Field:    "email",
						Function: common.Filter_EQUAL,
						Values:   []string{data.Email.ValueString()},
					},
				},
			},
			IncludeSupportStaff: true,
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to fetch user", err.Error())
			return
		}
		if len(users.Users) == 0 {
			resp.Diagnostics.AddError("User not found", fmt.Sprintf("User %s not found", data.Email.ValueString()))
			return
		}
		user = users.Users[0]
	} else if !data.Id.IsUnknown() {
		userResp, err := d.conn.GetUser(ctx, &identity.GetUserRequest{
			Id: &common.UserIdentifier{
				Subject: data.Id.ValueString(),
			},
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				resp.Diagnostics.AddError("User not found", fmt.Sprintf("User with ID %s not found", data.Id.ValueString()))
				return
			}
			resp.Diagnostics.AddError("Failed to fetch user", err.Error())
			return
		}
		user = userResp.User
	} else {
		resp.Diagnostics.AddError("Unknown user ID org email", "User ID and email are unknown. You must specify one")
		return
	}

	tflog.Trace(ctx, "Fetched user", map[string]interface{}{
		"user": user,
	})

	data.Id = types.StringValue(user.Id.Subject)
	data.FirstName = types.StringValue(user.Spec.FirstName)
	data.LastName = types.StringValue(user.Spec.LastName)
	data.Email = types.StringValue(user.Spec.Email)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

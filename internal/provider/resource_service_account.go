package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

var (
	_ resource.Resource                = (*serviceAccountResource)(nil)
	_ resource.ResourceWithConfigure   = (*serviceAccountResource)(nil)
	_ resource.ResourceWithImportState = (*serviceAccountResource)(nil)
)

func NewServiceAccountResource() resource.Resource {
	return &serviceAccountResource{}
}

type serviceAccountResource struct {
	client *client.Client
}

type serviceAccountModel struct {
	ID       types.String `tfsdk:"id"`
	Username types.String `tfsdk:"username"`
}

func (r *serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A MotherDuck service-account user (`POST /v1/users`). The API currently " +
			"creates users with the **Member** role only.\n\n" +
			"~> **Destroying this resource permanently deletes the user and all of their data. This cannot be undone.**",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `username`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Username for the service account. Changing it replaces the account.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *serviceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.CreateServiceAccount(ctx, plan.Username.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create service account", err.Error())
		return
	}

	plan.ID = types.StringValue(user.Username)
	plan.Username = types.StringValue(user.Username)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API has no GET /v1/users/{username}; probe existence via the token
	// list endpoint, which 404s when the user does not exist.
	_, err := r.client.ListTokens(ctx, state.Username.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read service account", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceAccountResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes force replacement; Update is never called.
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteUser(ctx, state.Username.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete service account", err.Error())
	}
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("username"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

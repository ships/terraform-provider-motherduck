package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

// usernameRegex matches MotherDuck's server-side rule: the API rejects anything
// but letters, numbers, and underscores. Validating here surfaces the problem at
// plan time instead of as a mid-apply HTTP 400.
var usernameRegex = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

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
	ID             types.String `tfsdk:"id"`
	Username       types.String `tfsdk:"username"`
	DeletionPolicy types.String `tfsdk:"deletion_policy"`
}

func (r *serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A MotherDuck service-account user (`POST /v1/users`). The API currently " +
			"creates users with the **Member** role only.\n\n" +
			"~> **`deletion_policy` defaults to `prevent`, so destroying is refused until you set it to " +
			"`cascade` (permanently deletes the user and all their data) or `retain` (drops it from state, " +
			"keeps the user). `cascade` cannot be undone.**",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `username`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Username for the service account. May contain only letters, numbers, " +
					"and underscores. Changing it replaces the account.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(usernameRegex,
						"must contain only letters, numbers, and underscores"),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"deletion_policy": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("prevent"),
				MarkdownDescription: "Behavior when this resource is destroyed: `prevent` (default) refuses the " +
					"destroy so Terraform keeps managing the account; `retain` removes it from Terraform state " +
					"while leaving the user in MotherDuck; `cascade` permanently deletes the user and all of " +
					"their data. Changing this value is an in-place update, not a replacement.",
				Validators: []validator.String{
					stringvalidator.OneOf("cascade", "retain", "prevent"),
				},
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

func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	switch state.DeletionPolicy.ValueString() {
	case "prevent":
		resp.Diagnostics.AddError(
			"Deletion prevented by deletion_policy",
			fmt.Sprintf("Service account %q has deletion_policy = %q. Set deletion_policy to "+
				"\"cascade\" or \"retain\" and apply before destroying.",
				state.Username.ValueString(), "prevent"),
		)
	case "retain":
		// Remove from state without deleting the user; the account survives in MotherDuck.
	case "cascade":
		err := r.client.DeleteUser(ctx, state.Username.ValueString())
		if err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Failed to delete service account", err.Error())
		}
	}
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("username"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deletion_policy"), "prevent")...)
}

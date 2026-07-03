package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

var (
	_ resource.Resource                = (*tokenResource)(nil)
	_ resource.ResourceWithConfigure   = (*tokenResource)(nil)
	_ resource.ResourceWithImportState = (*tokenResource)(nil)
)

func NewTokenResource() resource.Resource {
	return &tokenResource{}
}

type tokenResource struct {
	client *client.Client
}

type tokenModel struct {
	ID         types.String `tfsdk:"id"`
	Username   types.String `tfsdk:"username"`
	Name       types.String `tfsdk:"name"`
	TTLSeconds types.Int64  `tfsdk:"ttl_seconds"`
	TokenType  types.String `tfsdk:"token_type"`
	Token      types.String `tfsdk:"token"`
	CreatedAt  types.String `tfsdk:"created_at"`
	ExpiresAt  types.String `tfsdk:"expires_at"`
	ReadOnly   types.Bool   `tfsdk:"read_only"`
}

func (r *tokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (r *tokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An access token for a MotherDuck user (`POST /v1/users/{username}/tokens`). " +
			"The API has no token-update operation, so every change forces a new token.\n\n" +
			"-> The secret `token` value is only returned at creation time and is stored in Terraform state; " +
			"protect your state accordingly (or write it to a secret manager).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Token ID (UUID).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User the token belongs to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name for the token.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ttl_seconds": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Token lifetime in seconds (300 to 31536000). " +
					"Omit for a non-expiring token.",
				Validators:    []validator.Int64{int64validator.Between(300, 31536000)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"token_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(client.TokenTypeReadWrite)),
				MarkdownDescription: "Token type: `read_write` (default) or `read_scaling`.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(client.TokenTypeReadWrite), string(client.TokenTypeReadScaling)),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The secret token value. Only returned at creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"expires_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Expiry timestamp; empty for non-expiring tokens.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"read_only": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the token is read-only.",
			},
		},
	}
}

func (r *tokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateTokenRequest{
		Name:      plan.Name.ValueString(),
		TokenType: client.TokenType(plan.TokenType.ValueString()),
	}
	if !plan.TTLSeconds.IsNull() {
		ttl := plan.TTLSeconds.ValueInt64()
		createReq.TTL = &ttl
	}

	tok, err := r.client.CreateToken(ctx, plan.Username.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create token", err.Error())
		return
	}

	plan.ID = types.StringValue(tok.ID)
	plan.Token = types.StringValue(tok.Token)
	plan.CreatedAt = types.StringValue(tok.CreatedTS)
	plan.ExpiresAt = types.StringValue(tok.ExpireAt)
	plan.ReadOnly = types.BoolValue(tok.ReadOnly)
	plan.TokenType = types.StringValue(string(tok.TokenType))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tokens, err := r.client.ListTokens(ctx, state.Username.ValueString())
	if client.IsNotFound(err) {
		// The owning user is gone, so the token is too.
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read token", err.Error())
		return
	}

	for _, tok := range tokens {
		if tok.ID == state.ID.ValueString() {
			state.Name = types.StringValue(tok.Name)
			state.CreatedAt = types.StringValue(tok.CreatedTS)
			state.ExpiresAt = types.StringValue(tok.ExpireAt)
			state.ReadOnly = types.BoolValue(tok.ReadOnly)
			state.TokenType = types.StringValue(string(tok.TokenType))
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	// Token no longer exists (revoked or expired).
	resp.State.RemoveResource(ctx)
}

func (r *tokenResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes force replacement; Update is never called.
}

func (r *tokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteToken(ctx, state.Username.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete token", err.Error())
	}
}

// ImportState imports a token as "username/token_id". The secret token value
// cannot be recovered, so `token` stays null after import.
func (r *tokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`Import a token as "<username>/<token_id>", e.g. "svc_etl/9a1b...".`,
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("username"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

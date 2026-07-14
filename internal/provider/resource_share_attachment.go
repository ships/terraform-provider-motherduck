package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*shareAttachmentResource)(nil)
	_ resource.ResourceWithImportState = (*shareAttachmentResource)(nil)
)

func NewShareAttachmentResource() resource.Resource {
	return &shareAttachmentResource{}
}

type shareAttachmentResource struct {
	sqlBackedResource
}

type shareAttachmentModel struct {
	ID       types.String `tfsdk:"id"`
	ShareURL types.String `tfsdk:"share_url"`
	Alias    types.String `tfsdk:"alias"`
	Token    types.String `tfsdk:"token"`
}

// attachShareSQL composes ATTACH. The share URL is a single-quoted string
// literal; the alias is a double-quoted identifier.
func attachShareSQL(url, alias string) string {
	return "ATTACH " + quoteLiteral(url) + " AS " + quoteIdent(alias) + ";"
}

func detachSQL(alias string) string {
	return "DETACH " + quoteIdent(alias) + ";"
}

// listAttachedSharesSQL lists every attached MotherDuck share visible to the
// account. Read scans the `alias` column to detect this attachment's presence.
func listAttachedSharesSQL() string {
	return "SELECT alias, is_attached, type, fully_qualified_name " +
		"FROM MD_ALL_DATABASES() WHERE type = 'MotherDuck share' AND is_attached = true;"
}

func (r *shareAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_share_attachment"
}

func (r *shareAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	tokenAttr := r.tokenAttribute()
	tokenAttr.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
	tokenAttr.MarkdownDescription = "Data-plane token of the **consumer** account that will hold the attached " +
		"database (e.g. `motherduck_token.x.token`). `ATTACH` runs as this account. Changing it replaces the attachment."

	resp.Schema = schema.Schema{
		MarkdownDescription: "A consumer-side attachment of a MotherDuck share, managed via `ATTACH` / `DETACH` DDL " +
			"run over a data-plane SQL connection. The attachment is owned by the consumer account whose `token` is " +
			"set here — `ATTACH` binds the share URL to a local `alias` in that account.\n\n" +
			"~> **All attributes are replace-triggering: `ATTACH`/`DETACH` have no in-place mutation, so changing " +
			"`share_url`, `alias`, or `token` detaches and re-attaches.**",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `alias`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"share_url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The `md:_share/...` URL to attach (e.g. `motherduck_share.x.share_url`). Changing it replaces the attachment.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"alias": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Local database name the share is attached as. Changing it replaces the attachment.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"token": tokenAttr,
		},
	}
}

func (r *shareAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan shareAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alias := plan.Alias.ValueString()
	if err := r.clientFor(plan.Token.ValueString()).Exec(ctx, attachShareSQL(plan.ShareURL.ValueString(), alias)); err != nil {
		resp.Diagnostics.AddError("Failed to attach share", err.Error())
		return
	}

	plan.ID = types.StringValue(alias)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *shareAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state shareAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rows, closeDB, err := r.clientFor(state.Token.ValueString()).Query(ctx, listAttachedSharesSQL())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read share attachment", err.Error())
		return
	}
	defer func() { _ = closeDB() }()

	cols, err := rows.Columns()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read share attachment", err.Error())
		return
	}
	aliasIdx := -1
	for i, c := range cols {
		if c == "alias" {
			aliasIdx = i
			break
		}
	}
	if aliasIdx < 0 {
		resp.Diagnostics.AddError("Failed to read share attachment", "MD_ALL_DATABASES() has no `alias` column")
		return
	}

	want := state.Alias.ValueString()
	found := false
	for rows.Next() {
		cells := make([]any, len(cols))
		for i := range cells {
			cells[i] = new(any)
		}
		if err := rows.Scan(cells...); err != nil {
			resp.Diagnostics.AddError("Failed to read share attachment", err.Error())
			return
		}
		if alias, ok := (*cells[aliasIdx].(*any)).(string); ok && alias == want {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Failed to read share attachment", err.Error())
		return
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(want)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *shareAttachmentResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// share_url, alias, and token all force replacement; Update is never called.
}

func (r *shareAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state shareAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.clientFor(state.Token.ValueString()).Exec(ctx, detachSQL(state.Alias.ValueString())); err != nil {
		resp.Diagnostics.AddError("Failed to detach share", err.Error())
	}
}

func (r *shareAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("alias"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

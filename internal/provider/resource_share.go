package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*shareResource)(nil)
	_ resource.ResourceWithImportState = (*shareResource)(nil)
)

func NewShareResource() resource.Resource {
	return &shareResource{}
}

type shareResource struct {
	sqlBackedResource
}

type shareModel struct {
	Name       types.String `tfsdk:"name"`
	Database   types.String `tfsdk:"database"`
	Token      types.String `tfsdk:"token"`
	Access     types.String `tfsdk:"access"`
	UpdateMode types.String `tfsdk:"update_mode"`
	GrantTo    types.List   `tfsdk:"grant_to"`
	ShareURL   types.String `tfsdk:"share_url"`
}

// createShareSQL composes CREATE SHARE. access maps "restricted"->RESTRICTED and
// "unrestricted"->UNRESTRICTED; updateMode maps "automatic"->UPDATE AUTOMATIC (the
// share tracks source writes) and "manual"->UPDATE MANUAL (a static snapshot
// republished only by UPDATE SHARE). Any other value is a modeling error and
// returns an error rather than emitting unvalidated DDL. Legality of a
// (source, updateMode) pair is enforced by the caller, not here — a DuckLake
// admits only automatic mode.
func createShareSQL(name, database, access, updateMode string) (string, error) {
	var accessClause string
	switch access {
	case "restricted":
		accessClause = "ACCESS RESTRICTED"
	case "unrestricted":
		accessClause = "ACCESS UNRESTRICTED"
	default:
		return "", fmt.Errorf("unsupported share access %q: must be \"restricted\" or \"unrestricted\"", access)
	}
	var updateClause string
	switch updateMode {
	case "automatic":
		updateClause = "UPDATE AUTOMATIC"
	case "manual":
		updateClause = "UPDATE MANUAL"
	default:
		return "", fmt.Errorf("unsupported update mode %q: must be \"automatic\" or \"manual\"", updateMode)
	}
	return "CREATE SHARE " + quoteIdent(name) + " FROM " + quoteIdent(database) +
		" (" + accessClause + ", " + updateClause + ");", nil
}

// showDatabasesSQL lists every database visible to the account with its type.
// The type of a DuckLake carries the "ducklake" token (see isDuckLakeType); a
// plain database is bare "motherduck". This is the only catalog surface that
// distinguishes the two — duckdb_databases() reports "motherduck" for both.
func showDatabasesSQL() string {
	return "SHOW ALL DATABASES;"
}

// isDuckLakeType classifies a SHOW ALL DATABASES `type` value. MotherDuck reports
// a DuckLake as "motherduck ducklake" and a plain database as bare "motherduck".
func isDuckLakeType(dbType string) bool {
	return strings.Contains(dbType, "ducklake")
}

func dropShareSQL(name string) string {
	return "DROP SHARE " + quoteIdent(name) + ";"
}

// shareUrlSQL reads a single share's row from the OWNED_SHARES system view. The
// `url` column carries the md:_share/... identifier stored as share_url. The
// name filter is a string literal (quoteLiteral), not an identifier.
func shareUrlSQL(name string) string {
	return "SELECT name, url, access, visibility, source_db_name " +
		"FROM MD_INFORMATION_SCHEMA.OWNED_SHARES WHERE name = " + quoteLiteral(name) + ";"
}

// grantReadSQL grants READ on a RESTRICTED share to one or more accounts. Each
// username is a quoted identifier (an @-username must be double-quoted).
func grantReadSQL(name string, users []string) string {
	quoted := make([]string, len(users))
	for i, u := range users {
		quoted[i] = quoteIdent(u)
	}
	return "GRANT READ ON SHARE " + quoteIdent(name) + " TO " + strings.Join(quoted, ", ") + ";"
}

func revokeReadSQL(name, user string) string {
	return "REVOKE READ ON SHARE " + quoteIdent(name) + " FROM " + quoteIdent(user) + ";"
}

func (r *shareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_share"
}

func (r *shareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	tokenAttr := r.tokenAttribute()
	tokenAttr.MarkdownDescription = "Data-plane token of the account that owns the source database " +
		"(e.g. `motherduck_token.x.token`). `CREATE SHARE` runs as the owner of the shared database."

	resp.Schema = schema.Schema{
		MarkdownDescription: "A MotherDuck share of a database, managed via `CREATE SHARE` / `DROP SHARE` DDL run " +
			"over a data-plane SQL connection. MotherDuck exposes shares only over SQL, not the REST API, so this " +
			"resource authenticates with the source-database owner's `token`.\n\n" +
			"~> **Changing `name`, `database`, or `access` replaces the share and rotates its `share_url`; existing " +
			"consumers of the old URL disconnect. There is no `ALTER SHARE`, so an access change can only be applied " +
			"by `CREATE OR REPLACE SHARE`, which mints a new URL — `access` is therefore replace-triggering.**",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the share. Changing it replaces the share.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"database": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the source database to share. Changing it replaces the share.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"token": tokenAttr,
			"access": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("restricted"),
				MarkdownDescription: "Access model: `restricted` (owner only; extend with `grant_to`) or " +
					"`unrestricted` (all MotherDuck users in the same cloud region). Changing it replaces the share " +
					"because the only SQL path to change access is `CREATE OR REPLACE SHARE`, which rotates the URL.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"update_mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("automatic"),
				MarkdownDescription: "How source writes propagate to the share: `automatic` (default) tracks the " +
					"source and republishes committed changes within minutes; `manual` freezes the share as a static " +
					"snapshot that only `UPDATE SHARE` republishes. `manual` is valid only for a standard database — a " +
					"DuckLake source admits `automatic` only, and requesting `manual` for one is rejected at plan-apply. " +
					"Changing it replaces the share because update mode is fixed at `CREATE SHARE` and can only be " +
					"changed via `CREATE OR REPLACE SHARE`, which rotates the URL.",
				Validators:    []validator.String{stringvalidator.OneOf("automatic", "manual")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"grant_to": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
				MarkdownDescription: "Account usernames granted READ on this share. Applies only when `access` is " +
					"`restricted`. Not read back from the server (MotherDuck exposes no view of a share's grantees), " +
					"so it is authoritative from configuration and never shows drift from out-of-band grants.",
			},
			"share_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The `md:_share/<database>/<token>` URL consumers attach. Rotates on replace.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *shareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan shareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	access := plan.Access.ValueString()
	database := plan.Database.ValueString()
	updateMode := plan.UpdateMode.ValueString()
	client := r.clientFor(plan.Token.ValueString())

	// A DuckLake admits only automatic update mode. The source type is inspected
	// solely to reject the illegal (DuckLake, manual) pair — automatic is valid for
	// either source type and needs no inspection. This coupling of a config value
	// (update_mode) to a live catalog fact (source type) can only be resolved at
	// apply, so it surfaces here as a diagnostic rather than in schema validation.
	if updateMode == "manual" {
		ducklake, err := databaseIsDuckLake(ctx, client, database)
		if err != nil {
			resp.Diagnostics.AddError("Failed to inspect source database", err.Error())
			return
		}
		if ducklake {
			resp.Diagnostics.AddAttributeError(path.Root("update_mode"),
				"Manual update mode unsupported for a DuckLake source",
				"A DuckLake can only be shared in automatic update mode. Set update_mode to \"automatic\" "+
					"(the default) or omit it.")
			return
		}
	}

	createSQL, err := createShareSQL(name, database, access, updateMode)
	if err != nil {
		resp.Diagnostics.AddError("Invalid share configuration", err.Error())
		return
	}
	if err := client.Exec(ctx, createSQL); err != nil {
		resp.Diagnostics.AddError("Failed to create share", err.Error())
		return
	}

	var grantTo []string
	resp.Diagnostics.Append(plan.GrantTo.ElementsAs(ctx, &grantTo, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if access == "restricted" && len(grantTo) > 0 {
		if err := client.Exec(ctx, grantReadSQL(name, grantTo)); err != nil {
			resp.Diagnostics.AddError("Failed to grant read on share", err.Error())
			return
		}
	}

	url, found, err := r.readShareURL(ctx, client, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read share URL", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read share URL", "share not found in OWNED_SHARES immediately after create")
		return
	}
	plan.ShareURL = types.StringValue(url)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *shareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state shareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url, found, err := r.readShareURL(ctx, r.clientFor(state.Token.ValueString()), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read share", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	// grant_to is intentionally not refreshed: MotherDuck exposes no system view
	// of a share's grantees, so it is left at the prior state value to avoid
	// perpetual drift. It is authoritative from configuration only.
	state.ShareURL = types.StringValue(url)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *shareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state shareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// name/database/access are RequiresReplace; grant_to is the only in-place
	// mutation. Diff old vs new grantees and apply GRANT/REVOKE accordingly.
	var oldUsers, newUsers []string
	resp.Diagnostics.Append(state.GrantTo.ElementsAs(ctx, &oldUsers, false)...)
	resp.Diagnostics.Append(plan.GrantTo.ElementsAs(ctx, &newUsers, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// access is RequiresReplace, so plan and state agree here. Grants apply only
	// to restricted shares; an unrestricted share grants to everyone in-region and
	// GRANT/REVOKE on it is a server error, so grant_to is inert and skipped.
	if plan.Access.ValueString() != "restricted" {
		plan.ShareURL = state.ShareURL
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	name := plan.Name.ValueString()
	client := r.clientFor(plan.Token.ValueString())

	oldSet := make(map[string]struct{}, len(oldUsers))
	for _, u := range oldUsers {
		oldSet[u] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newUsers))
	for _, u := range newUsers {
		newSet[u] = struct{}{}
	}

	var added []string
	for _, u := range newUsers {
		if _, ok := oldSet[u]; !ok {
			added = append(added, u)
		}
	}
	if len(added) > 0 {
		if err := client.Exec(ctx, grantReadSQL(name, added)); err != nil {
			resp.Diagnostics.AddError("Failed to grant read on share", err.Error())
			return
		}
	}
	for _, u := range oldUsers {
		if _, ok := newSet[u]; !ok {
			if err := client.Exec(ctx, revokeReadSQL(name, u)); err != nil {
				resp.Diagnostics.AddError("Failed to revoke read on share", err.Error())
				return
			}
		}
	}

	// share_url is unaffected by grants; carry it forward from prior state.
	plan.ShareURL = state.ShareURL
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *shareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state shareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.clientFor(state.Token.ValueString()).Exec(ctx, dropShareSQL(state.Name.ValueString())); err != nil {
		resp.Diagnostics.AddError("Failed to delete share", err.Error())
	}
}

func (r *shareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	token, name, ok := splitImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", "expected `<token>,<share-name>`")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("token"), token)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

// databaseIsDuckLake reports whether the named source database is a DuckLake, by
// scanning SHOW ALL DATABASES for the matching `alias`. A DuckLake must be shared
// in automatic update mode; a source absent from the listing is treated as a
// plain database (false) rather than an error — CREATE SHARE then surfaces any
// genuine "database not found" from the server with its own diagnostics.
func databaseIsDuckLake(ctx context.Context, client sqlQuerier, database string) (bool, error) {
	rows, closeDB, err := client.Query(ctx, showDatabasesSQL())
	if err != nil {
		return false, err
	}
	defer func() { _ = closeDB() }()

	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	aliasIdx, typeIdx := -1, -1
	for i, c := range cols {
		switch c {
		case "alias":
			aliasIdx = i
		case "type":
			typeIdx = i
		}
	}
	if aliasIdx < 0 || typeIdx < 0 {
		return false, fmt.Errorf("SHOW ALL DATABASES lacks `alias`/`type` columns")
	}

	for rows.Next() {
		cells := make([]any, len(cols))
		for i := range cells {
			cells[i] = new(any)
		}
		if err := rows.Scan(cells...); err != nil {
			return false, err
		}
		alias, aliasOK := (*cells[aliasIdx].(*any)).(string)
		dbType, typeOK := (*cells[typeIdx].(*any)).(string)
		if aliasOK && typeOK && alias == database {
			return isDuckLakeType(dbType), nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// readShareURL runs shareUrlSQL and returns the `url` column of the single
// matching row. found is false when the share is absent (drift / deleted).
func (r *shareResource) readShareURL(ctx context.Context, client sqlQuerier, name string) (url string, found bool, err error) {
	rows, closeDB, err := client.Query(ctx, shareUrlSQL(name))
	if err != nil {
		return "", false, err
	}
	defer func() { _ = closeDB() }()

	cols, err := rows.Columns()
	if err != nil {
		return "", false, err
	}
	urlIdx := -1
	for i, c := range cols {
		if c == "url" {
			urlIdx = i
			break
		}
	}
	if urlIdx < 0 {
		return "", false, fmt.Errorf("OWNED_SHARES has no `url` column")
	}

	if rows.Next() {
		cells := make([]any, len(cols))
		for i := range cells {
			cells[i] = new(any)
		}
		if err := rows.Scan(cells...); err != nil {
			return "", false, err
		}
		if v, ok := (*cells[urlIdx].(*any)).(string); ok {
			url = v
		}
		found = true
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}
	return url, found, nil
}

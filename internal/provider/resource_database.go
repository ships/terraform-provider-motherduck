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
	_ resource.Resource                = (*databaseResource)(nil)
	_ resource.ResourceWithImportState = (*databaseResource)(nil)
)

func NewDatabaseResource() resource.Resource {
	return &databaseResource{}
}

type databaseResource struct {
	sqlBackedResource
}

type databaseModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Token types.String `tfsdk:"token"`
}

func createDatabaseSQL(name string) string {
	return "CREATE DATABASE " + quoteIdent(name) + ";"
}

func dropDatabaseSQL(name string) string {
	return "DROP DATABASE " + quoteIdent(name) + ";"
}

// listDatabasesSQL lists every database visible to the account. The database
// name is the `alias` column of the result; Read scans it to detect drift.
func listDatabasesSQL() string {
	return "SHOW ALL DATABASES;"
}

func (r *databaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (r *databaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	tokenAttr := r.tokenAttribute()
	tokenAttr.MarkdownDescription = "Data-plane token of the account that owns this database " +
		"(e.g. `motherduck_token.x.token`). The `CREATE DATABASE` DDL runs as this account."

	resp.Schema = schema.Schema{
		MarkdownDescription: "A MotherDuck database, managed via `CREATE DATABASE` / `DROP DATABASE` DDL run " +
			"over a data-plane SQL connection. MotherDuck exposes databases only over SQL, not the REST API, " +
			"so this resource authenticates with a per-resource account `token` rather than the provider's admin token.\n\n" +
			"~> **Destroying this resource drops the database. `DROP DATABASE` uses the default `RESTRICT`, " +
			"so the delete fails if a share was created from this database.**",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `name`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the database. Changing it replaces the database.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"token": tokenAttr,
		},
	}
}

func (r *databaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	if err := r.clientFor(plan.Token.ValueString()).Exec(ctx, createDatabaseSQL(name)); err != nil {
		resp.Diagnostics.AddError("Failed to create database", err.Error())
		return
	}

	plan.ID = types.StringValue(name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rows, closeDB, err := r.clientFor(state.Token.ValueString()).Query(ctx, listDatabasesSQL())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read database", err.Error())
		return
	}
	defer func() { _ = closeDB() }()

	cols, err := rows.Columns()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read database", err.Error())
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
		resp.Diagnostics.AddError("Failed to read database", "SHOW ALL DATABASES has no `alias` column")
		return
	}

	want := state.Name.ValueString()
	found := false
	for rows.Next() {
		cells := make([]any, len(cols))
		for i := range cells {
			cells[i] = new(any)
		}
		if err := rows.Scan(cells...); err != nil {
			resp.Diagnostics.AddError("Failed to read database", err.Error())
			return
		}
		if alias, ok := (*cells[aliasIdx].(*any)).(string); ok && alias == want {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Failed to read database", err.Error())
		return
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(want)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// name forces replacement and token is not part of the object's identity;
	// Update is never called.
}

func (r *databaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.clientFor(state.Token.ValueString()).Exec(ctx, dropDatabaseSQL(state.Name.ValueString())); err != nil {
		resp.Diagnostics.AddError("Failed to delete database", err.Error())
	}
}

func (r *databaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

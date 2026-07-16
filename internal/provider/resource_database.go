package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Token          types.String `tfsdk:"token"`
	DeletionPolicy types.String `tfsdk:"deletion_policy"`
}

func createDatabaseSQL(name string) string {
	return "CREATE DATABASE " + quoteIdent(name) + ";"
}

func dropDatabaseCascadeSQL(name string) string {
	return "DROP DATABASE " + quoteIdent(name) + " CASCADE;"
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
			"~> **`deletion_policy` defaults to `prevent`, so destroying is refused until you set it to " +
			"`cascade` (`DROP DATABASE ... CASCADE`) or `retain` (drops it from state, keeps the database).**",
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
			"deletion_policy": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("prevent"),
				MarkdownDescription: "Behavior when this resource is destroyed: `prevent` (default) refuses the " +
					"destroy; `retain` removes it from Terraform state while leaving the database in MotherDuck; " +
					"`cascade` runs `DROP DATABASE ... CASCADE`, dropping the database and its dependents. " +
					"Changing this value is an in-place update, not a replacement.",
				Validators: []validator.String{
					stringvalidator.OneOf("cascade", "retain", "prevent"),
				},
			},
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

func (r *databaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan databaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	switch state.DeletionPolicy.ValueString() {
	case "prevent":
		resp.Diagnostics.AddError(
			"Deletion prevented by deletion_policy",
			fmt.Sprintf("Database %q has deletion_policy = %q. Set deletion_policy to "+
				"\"cascade\" or \"retain\" and apply before destroying.",
				state.Name.ValueString(), "prevent"),
		)
	case "retain":
		// Remove from state without running DROP DATABASE; the database survives in MotherDuck.
	case "cascade":
		if err := r.clientFor(state.Token.ValueString()).Exec(ctx, dropDatabaseCascadeSQL(state.Name.ValueString())); err != nil {
			resp.Diagnostics.AddError("Failed to delete database", err.Error())
		}
	}
}

func (r *databaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	token, name, ok := splitImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", "expected `<token>,<database-name>`")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("token"), token)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deletion_policy"), "prevent")...)
}

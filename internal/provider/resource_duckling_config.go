package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

var instanceSizes = []string{"pulse", "standard", "jumbo", "mega", "giga"}

var (
	_ resource.Resource                = (*ducklingConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*ducklingConfigResource)(nil)
	_ resource.ResourceWithImportState = (*ducklingConfigResource)(nil)
)

func NewDucklingConfigResource() resource.Resource {
	return &ducklingConfigResource{}
}

type ducklingConfigResource struct {
	client *client.Client
}

type readWriteModel struct {
	InstanceSize    types.String `tfsdk:"instance_size"`
	CooldownSeconds types.Int64  `tfsdk:"cooldown_seconds"`
}

type readScalingModel struct {
	InstanceSize    types.String  `tfsdk:"instance_size"`
	FlockSize       types.Float64 `tfsdk:"flock_size"`
	CooldownSeconds types.Int64   `tfsdk:"cooldown_seconds"`
}

type ducklingConfigModel struct {
	ID          types.String      `tfsdk:"id"`
	Username    types.String      `tfsdk:"username"`
	ReadWrite   *readWriteModel   `tfsdk:"read_write"`
	ReadScaling *readScalingModel `tfsdk:"read_scaling"`
}

func (r *ducklingConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_duckling_config"
}

func (r *ducklingConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Duckling (compute instance) configuration for a user " +
			"(`PUT /v1/users/{username}/instances`). Requires the Admin role.\n\n" +
			"-> This is a settings-style resource: the configuration exists for every user, so " +
			"destroying it only removes it from Terraform state — it does not reset the user's " +
			"configuration in MotherDuck.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `username`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User whose Duckling configuration is managed.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
		Blocks: map[string]schema.Block{
			"read_write": schema.SingleNestedBlock{
				MarkdownDescription: "Configuration of the user's read/write Duckling.",
				Attributes: map[string]schema.Attribute{
					"instance_size": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Instance size: `pulse`, `standard`, `jumbo`, `mega`, or `giga`.",
						Validators:          []validator.String{stringvalidator.OneOf(instanceSizes...)},
					},
					"cooldown_seconds": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Idle seconds before the instance spins down (60 to 86400).",
						Validators:          []validator.Int64{int64validator.Between(60, 86400)},
					},
				},
			},
			"read_scaling": schema.SingleNestedBlock{
				MarkdownDescription: "Configuration of the user's read-scaling Ducklings.",
				Attributes: map[string]schema.Attribute{
					"instance_size": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Instance size: `pulse`, `standard`, `jumbo`, `mega`, or `giga`.",
						Validators:          []validator.String{stringvalidator.OneOf(instanceSizes...)},
					},
					"flock_size": schema.Float64Attribute{
						Required:            true,
						MarkdownDescription: "Number of read-scaling instances (0 to 64).",
						Validators:          []validator.Float64{float64validator.Between(0, 64)},
					},
					"cooldown_seconds": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Idle seconds before instances spin down (60 to 86400).",
						Validators:          []validator.Int64{int64validator.Between(60, 86400)},
					},
				},
			},
		},
	}
}

func (r *ducklingConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m *ducklingConfigModel) toAPI() client.DucklingConfig {
	cfg := client.DucklingConfig{}
	if m.ReadWrite != nil {
		cfg.ReadWrite.InstanceSize = m.ReadWrite.InstanceSize.ValueString()
		if !m.ReadWrite.CooldownSeconds.IsNull() {
			v := m.ReadWrite.CooldownSeconds.ValueInt64()
			cfg.ReadWrite.CooldownSeconds = &v
		}
	}
	if m.ReadScaling != nil {
		cfg.ReadScaling.InstanceSize = m.ReadScaling.InstanceSize.ValueString()
		cfg.ReadScaling.FlockSize = m.ReadScaling.FlockSize.ValueFloat64()
		if !m.ReadScaling.CooldownSeconds.IsNull() {
			v := m.ReadScaling.CooldownSeconds.ValueInt64()
			cfg.ReadScaling.CooldownSeconds = &v
		}
	}
	return cfg
}

func (m *ducklingConfigModel) fromAPI(cfg *client.DucklingConfig) {
	rw := &readWriteModel{
		InstanceSize:    types.StringValue(cfg.ReadWrite.InstanceSize),
		CooldownSeconds: types.Int64Null(),
	}
	if cfg.ReadWrite.CooldownSeconds != nil {
		rw.CooldownSeconds = types.Int64Value(*cfg.ReadWrite.CooldownSeconds)
	}
	rs := &readScalingModel{
		InstanceSize:    types.StringValue(cfg.ReadScaling.InstanceSize),
		FlockSize:       types.Float64Value(cfg.ReadScaling.FlockSize),
		CooldownSeconds: types.Int64Null(),
	}
	if cfg.ReadScaling.CooldownSeconds != nil {
		rs.CooldownSeconds = types.Int64Value(*cfg.ReadScaling.CooldownSeconds)
	}
	m.ReadWrite = rw
	m.ReadScaling = rs
}

func (r *ducklingConfigResource) apply(ctx context.Context, plan *ducklingConfigModel) (*client.DucklingConfig, error) {
	if plan.ReadWrite == nil || plan.ReadScaling == nil {
		return nil, fmt.Errorf("both read_write and read_scaling blocks are required by the MotherDuck API")
	}
	return r.client.SetDucklingConfig(ctx, plan.Username.ValueString(), plan.toAPI())
}

func (r *ducklingConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ducklingConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.apply(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to set Duckling configuration", err.Error())
		return
	}

	plan.ID = plan.Username
	plan.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ducklingConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ducklingConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.GetDucklingConfig(ctx, state.Username.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Duckling configuration", err.Error())
		return
	}

	state.ID = state.Username
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ducklingConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ducklingConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.apply(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Duckling configuration", err.Error())
		return
	}

	plan.ID = plan.Username
	plan.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ducklingConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// The API has no way to reset instance configuration to defaults, so delete
	// only removes the resource from state (documented in the schema).
}

func (r *ducklingConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("username"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

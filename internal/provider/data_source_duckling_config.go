package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/jpig18/terraform-provider-motherduck/internal/client"
)

var (
	_ datasource.DataSource              = (*ducklingConfigDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ducklingConfigDataSource)(nil)
)

func NewDucklingConfigDataSource() datasource.DataSource {
	return &ducklingConfigDataSource{}
}

type ducklingConfigDataSource struct {
	client *client.Client
}

type ducklingConfigDataModel struct {
	ID          types.String      `tfsdk:"id"`
	Username    types.String      `tfsdk:"username"`
	ReadWrite   *readWriteModel   `tfsdk:"read_write"`
	ReadScaling *readScalingModel `tfsdk:"read_scaling"`
}

func (d *ducklingConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_duckling_config"
}

func (d *ducklingConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Duckling (compute instance) configuration of a user " +
			"(`GET /v1/users/{username}/instances`). Requires the Admin role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `username`.",
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User whose Duckling configuration to read.",
			},
			"read_write": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"instance_size":    schema.StringAttribute{Computed: true},
					"cooldown_seconds": schema.Int64Attribute{Computed: true},
				},
			},
			"read_scaling": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"instance_size":    schema.StringAttribute{Computed: true},
					"flock_size":       schema.Float64Attribute{Computed: true},
					"cooldown_seconds": schema.Int64Attribute{Computed: true},
				},
			},
		},
	}
}

func (d *ducklingConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *ducklingConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ducklingConfigDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := d.client.GetDucklingConfig(ctx, config.Username.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Duckling configuration", err.Error())
		return
	}

	config.ID = config.Username
	rw := &readWriteModel{
		InstanceSize:    types.StringValue(got.ReadWrite.InstanceSize),
		CooldownSeconds: types.Int64Null(),
	}
	if got.ReadWrite.CooldownSeconds != nil {
		rw.CooldownSeconds = types.Int64Value(*got.ReadWrite.CooldownSeconds)
	}
	rs := &readScalingModel{
		InstanceSize:    types.StringValue(got.ReadScaling.InstanceSize),
		FlockSize:       types.Float64Value(got.ReadScaling.FlockSize),
		CooldownSeconds: types.Int64Null(),
	}
	if got.ReadScaling.CooldownSeconds != nil {
		rs.CooldownSeconds = types.Int64Value(*got.ReadScaling.CooldownSeconds)
	}
	config.ReadWrite = rw
	config.ReadScaling = rs

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

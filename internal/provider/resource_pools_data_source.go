package provider

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ datasource.DataSource = &ResourcePoolsDataSource{}

type ResourcePoolsDataSource struct {
	client *client.Client
}

type ResourcePoolsDataSourceModel struct {
	BusinessGroupID types.String `tfsdk:"business_group_id"`
	SourceKey       types.String `tfsdk:"source_key"`
	NodeType        types.String `tfsdk:"node_type"`
	ID              types.String `tfsdk:"id"`
	Total           types.Int64  `tfsdk:"total"`
	RawJSON         types.String `tfsdk:"raw_json"`
	Items           types.List   `tfsdk:"items"`
}

type ResourcePoolItemModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	CloudEntryTypeID types.String `tfsdk:"cloud_entry_type_id"`
	CloudEntryType   types.String `tfsdk:"cloud_entry_type"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	ReadOnly         types.Bool   `tfsdk:"read_only"`
	RawJSON          types.String `tfsdk:"raw_json"`
}

func NewResourcePoolsDataSource() datasource.DataSource {
	return &ResourcePoolsDataSource{}
}

func (d *ResourcePoolsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_pools"
}

func (d *ResourcePoolsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List SmartCMP resource pools for a business group, source key, and node type.",
		Attributes: map[string]schema.Attribute{
			"business_group_id": schema.StringAttribute{Required: true},
			"source_key":        schema.StringAttribute{Required: true},
			"node_type":         schema.StringAttribute{Required: true},
			"id":                schema.StringAttribute{Computed: true},
			"total":             schema.Int64Attribute{Computed: true},
			"raw_json":          schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                  schema.StringAttribute{Computed: true},
						"name":                schema.StringAttribute{Computed: true},
						"cloud_entry_type_id": schema.StringAttribute{Computed: true},
						"cloud_entry_type":    schema.StringAttribute{Computed: true},
						"enabled":             schema.BoolAttribute{Computed: true},
						"read_only":           schema.BoolAttribute{Computed: true},
						"raw_json":            schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ResourcePoolsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ResourcePoolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ResourcePoolsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("businessGroupId", data.BusinessGroupID.ValueString())
	params.Set("componentType", data.SourceKey.ValueString())
	params.Set("nodeType", data.NodeType.ValueString())
	params.Set("cloudEntryTypeId", "")
	params.Set("enabled", "true")
	params.Set("readOnly", "false")
	params.Set("strategy", "RB_POLICY_STATIC")

	var raw any
	if err := d.client.GetJSON(ctx, "/resource-bundles", params, &raw); err != nil {
		resp.Diagnostics.AddError("Read resource pools", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]ResourcePoolItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, ResourcePoolItemModel{
			ID:               types.StringValue(findFirstString(item, "id")),
			Name:             types.StringValue(findFirstString(item, "name")),
			CloudEntryTypeID: types.StringValue(findFirstString(item, "cloudEntryTypeId")),
			CloudEntryType:   types.StringValue(findFirstString(item, "cloudEntryType")),
			Enabled:          types.BoolValue(boolValue(item["enabled"])),
			ReadOnly:         types.BoolValue(boolValue(item["readOnly"])),
			RawJSON:          jsonStringValue(item),
		})
	}

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":                  types.StringType,
		"name":                types.StringType,
		"cloud_entry_type_id": types.StringType,
		"cloud_entry_type":    types.StringType,
		"enabled":             types.BoolType,
		"read_only":           types.BoolType,
		"raw_json":            types.StringType,
	}}
	listValue, diags := listValueFromStructs(ctx, objectType, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = hashDataSourceID("resource_pools", data.BusinessGroupID.ValueString(), data.SourceKey.ValueString(), data.NodeType.ValueString())
	data.Total = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

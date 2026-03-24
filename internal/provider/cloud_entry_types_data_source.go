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

var _ datasource.DataSource = &CloudEntryTypesDataSource{}

type CloudEntryTypesDataSource struct {
	client *client.Client
}

type CloudEntryTypesDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Total   types.Int64  `tfsdk:"total"`
	RawJSON types.String `tfsdk:"raw_json"`
	Items   types.List   `tfsdk:"items"`
}

type CloudEntryTypeItemModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Group   types.String `tfsdk:"group"`
	RawJSON types.String `tfsdk:"raw_json"`
}

func NewCloudEntryTypesDataSource() datasource.DataSource {
	return &CloudEntryTypesDataSource{}
}

func (d *CloudEntryTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_entry_types"
}

func (d *CloudEntryTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List SmartCMP cloud entry types for the current tenant.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true},
			"total":    schema.Int64Attribute{Computed: true},
			"raw_json": schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":       schema.StringAttribute{Computed: true},
						"name":     schema.StringAttribute{Computed: true},
						"group":    schema.StringAttribute{Computed: true},
						"raw_json": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *CloudEntryTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *CloudEntryTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CloudEntryTypesDataSourceModel

	params := url.Values{}
	params.Set("queryByCurrentTenant", "")

	var raw any
	if err := d.client.GetJSON(ctx, "/cloudentry-types/list_cloud_entry_types", params, &raw); err != nil {
		resp.Diagnostics.AddError("Read cloud entry types", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]CloudEntryTypeItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, CloudEntryTypeItemModel{
			ID:      types.StringValue(findFirstString(item, "id")),
			Name:    types.StringValue(findFirstString(item, "nameZh", "name")),
			Group:   types.StringValue(findFirstString(item, "group")),
			RawJSON: jsonStringValue(item),
		})
	}

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":       types.StringType,
		"name":     types.StringType,
		"group":    types.StringType,
		"raw_json": types.StringType,
	}}
	listValue, diags := listValueFromStructs(ctx, objectType, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = hashDataSourceID("cloud_entry_types")
	data.Total = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

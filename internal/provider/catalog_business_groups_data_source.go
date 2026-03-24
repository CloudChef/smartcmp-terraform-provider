package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ datasource.DataSource = &CatalogBusinessGroupsDataSource{}

type CatalogBusinessGroupsDataSource struct {
	client *client.Client
}

type CatalogBusinessGroupsDataSourceModel struct {
	CatalogID types.String `tfsdk:"catalog_id"`
	ID        types.String `tfsdk:"id"`
	Total     types.Int64  `tfsdk:"total"`
	RawJSON   types.String `tfsdk:"raw_json"`
	Items     types.List   `tfsdk:"items"`
}

type CatalogBusinessGroupItemModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	RawJSON     types.String `tfsdk:"raw_json"`
}

func NewCatalogBusinessGroupsDataSource() datasource.DataSource {
	return &CatalogBusinessGroupsDataSource{}
}

func (d *CatalogBusinessGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_business_groups"
}

func (d *CatalogBusinessGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available business groups for a SmartCMP catalog.",
		Attributes: map[string]schema.Attribute{
			"catalog_id": schema.StringAttribute{Required: true},
			"id":         schema.StringAttribute{Computed: true},
			"total":      schema.Int64Attribute{Computed: true},
			"raw_json":   schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"name":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"raw_json":    schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *CatalogBusinessGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *CatalogBusinessGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CatalogBusinessGroupsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var raw any
	if err := d.client.GetJSON(ctx, fmt.Sprintf("/catalogs/%s/available-bgs", data.CatalogID.ValueString()), nil, &raw); err != nil {
		resp.Diagnostics.AddError("Read catalog business groups", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]CatalogBusinessGroupItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, CatalogBusinessGroupItemModel{
			ID:          types.StringValue(findFirstString(item, "id")),
			Name:        types.StringValue(findFirstString(item, "name")),
			Description: types.StringValue(findFirstString(item, "description")),
			RawJSON:     jsonStringValue(item),
		})
	}

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":          types.StringType,
		"name":        types.StringType,
		"description": types.StringType,
		"raw_json":    types.StringType,
	}}
	listValue, diags := listValueFromStructs(ctx, objectType, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = hashDataSourceID("catalog_business_groups", data.CatalogID.ValueString())
	data.Total = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

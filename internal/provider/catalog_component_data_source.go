package provider

import (
	"context"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ datasource.DataSource = &CatalogComponentDataSource{}

type CatalogComponentDataSource struct {
	client *client.Client
}

type CatalogComponentDataSourceModel struct {
	SourceKey         types.String `tfsdk:"source_key"`
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	TypeName          types.String `tfsdk:"type_name"`
	Node              types.String `tfsdk:"node"`
	CloudEntryTypeIDs types.List   `tfsdk:"cloud_entry_type_ids"`
	RawJSON           types.String `tfsdk:"raw_json"`
}

func NewCatalogComponentDataSource() datasource.DataSource {
	return &CatalogComponentDataSource{}
}

func (d *CatalogComponentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_component"
}

func (d *CatalogComponentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Get component metadata for a SmartCMP catalog source key.",
		Attributes: map[string]schema.Attribute{
			"source_key": schema.StringAttribute{Required: true},
			"id":         schema.StringAttribute{Computed: true},
			"name":       schema.StringAttribute{Computed: true},
			"type_name":  schema.StringAttribute{Computed: true},
			"node":       schema.StringAttribute{Computed: true},
			"cloud_entry_type_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
			"raw_json": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *CatalogComponentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *CatalogComponentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CatalogComponentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("resourceType", data.SourceKey.ValueString())

	var raw any
	if err := d.client.GetJSON(ctx, "/components", params, &raw); err != nil {
		resp.Diagnostics.AddError("Read catalog component", err.Error())
		return
	}

	items := extractItems(raw)
	if len(items) == 0 {
		resp.Diagnostics.AddError("Read catalog component", "no component returned for source_key")
		return
	}
	item := items[0]
	model := asMap(item["model"])
	typeName := findFirstString(model, "typeName")
	if typeName == "" {
		typeName = findFirstString(item, "typeName", "type")
	}
	node := ""
	if parts := stringSliceValue(strings.ReplaceAll(typeName, ".", ",")); len(parts) > 0 {
		node = parts[len(parts)-1]
	}

	cloudEntryTypeIDs := stringSliceValue(findFirstString(model, "cloudEntryTypeIds"))
	listValue, diags := types.ListValueFrom(ctx, types.StringType, cloudEntryTypeIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(findFirstString(item, "id"))
	data.Name = types.StringValue(findFirstString(item, "nameZh", "name"))
	data.TypeName = types.StringValue(typeName)
	data.Node = types.StringValue(node)
	data.CloudEntryTypeIDs = listValue
	data.RawJSON = jsonStringValue(item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

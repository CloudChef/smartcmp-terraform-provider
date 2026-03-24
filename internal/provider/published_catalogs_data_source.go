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

var _ datasource.DataSource = &PublishedCatalogsDataSource{}

type PublishedCatalogsDataSource struct {
	client *client.Client
}

type PublishedCatalogsDataSourceModel struct {
	Query   types.String `tfsdk:"query"`
	ID      types.String `tfsdk:"id"`
	Total   types.Int64  `tfsdk:"total"`
	RawJSON types.String `tfsdk:"raw_json"`
	Items   types.List   `tfsdk:"items"`
}

type PublishedCatalogItemModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	SourceKey       types.String `tfsdk:"source_key"`
	ServiceCategory types.String `tfsdk:"service_category"`
	Description     types.String `tfsdk:"description"`
	RawJSON         types.String `tfsdk:"raw_json"`
}

func NewPublishedCatalogsDataSource() datasource.DataSource {
	return &PublishedCatalogsDataSource{}
}

func (d *PublishedCatalogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_published_catalogs"
}

func (d *PublishedCatalogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List published SmartCMP service catalogs.",
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				MarkdownDescription: "Optional keyword filter applied to catalog names.",
				Optional:            true,
			},
			"id":       schema.StringAttribute{Computed: true},
			"total":    schema.Int64Attribute{Computed: true},
			"raw_json": schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":               schema.StringAttribute{Computed: true},
						"name":             schema.StringAttribute{Computed: true},
						"source_key":       schema.StringAttribute{Computed: true},
						"service_category": schema.StringAttribute{Computed: true},
						"description":      schema.StringAttribute{Computed: true},
						"raw_json":         schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *PublishedCatalogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *PublishedCatalogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PublishedCatalogsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("query", "")
	params.Set("states", "PUBLISHED")
	params.Set("page", "1")
	params.Set("size", "50")
	params.Set("sort", "catalogIndex,asc")
	if !data.Query.IsNull() && !data.Query.IsUnknown() && data.Query.ValueString() != "" {
		params.Set("queryValue", data.Query.ValueString())
	}

	var raw any
	if err := d.client.GetJSON(ctx, "/catalogs/published", params, &raw); err != nil {
		resp.Diagnostics.AddError("Read published catalogs", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]PublishedCatalogItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, PublishedCatalogItemModel{
			ID:              types.StringValue(findFirstString(item, "id")),
			Name:            types.StringValue(findFirstString(item, "nameZh", "name")),
			SourceKey:       types.StringValue(findFirstString(item, "sourceKey")),
			ServiceCategory: types.StringValue(findFirstString(item, "serviceCategory")),
			Description:     types.StringValue(findFirstString(item, "instructions", "description")),
			RawJSON:         jsonStringValue(item),
		})
	}

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":               types.StringType,
		"name":             types.StringType,
		"source_key":       types.StringType,
		"service_category": types.StringType,
		"description":      types.StringType,
		"raw_json":         types.StringType,
	}}
	listValue, diags := listValueFromStructs(ctx, objectType, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryValue := ""
	if !data.Query.IsNull() && !data.Query.IsUnknown() {
		queryValue = data.Query.ValueString()
	}
	data.ID = hashDataSourceID("published_catalogs", queryValue)
	data.Total = types.Int64Value(extractTotal(raw, len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

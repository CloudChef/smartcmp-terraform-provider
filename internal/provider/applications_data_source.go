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

var _ datasource.DataSource = &ApplicationsDataSource{}

type ApplicationsDataSource struct {
	client *client.Client
}

type ApplicationsDataSourceModel struct {
	BusinessGroupID types.String `tfsdk:"business_group_id"`
	Query           types.String `tfsdk:"query"`
	ID              types.String `tfsdk:"id"`
	Total           types.Int64  `tfsdk:"total"`
	RawJSON         types.String `tfsdk:"raw_json"`
	Items           types.List   `tfsdk:"items"`
}

type ApplicationItemModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	RawJSON     types.String `tfsdk:"raw_json"`
}

func NewApplicationsDataSource() datasource.DataSource {
	return &ApplicationsDataSource{}
}

func (d *ApplicationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_applications"
}

func (d *ApplicationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List SmartCMP applications or top-level groups for a business group.",
		Attributes: map[string]schema.Attribute{
			"business_group_id": schema.StringAttribute{Required: true},
			"query":             schema.StringAttribute{Optional: true},
			"id":                schema.StringAttribute{Computed: true},
			"total":             schema.Int64Attribute{Computed: true},
			"raw_json":          schema.StringAttribute{Computed: true},
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

func (d *ApplicationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ApplicationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApplicationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("query", "")
	params.Set("topGroup", "true")
	params.Set("businessGroupIds", data.BusinessGroupID.ValueString())
	params.Set("page", "1")
	params.Set("size", "50")
	params.Set("sort", "name,asc")
	if !data.Query.IsNull() && !data.Query.IsUnknown() && data.Query.ValueString() != "" {
		params.Set("queryValue", data.Query.ValueString())
	}

	var raw any
	if err := d.client.GetJSON(ctx, "/groups", params, &raw); err != nil {
		resp.Diagnostics.AddError("Read applications", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]ApplicationItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, ApplicationItemModel{
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

	queryValue := ""
	if !data.Query.IsNull() && !data.Query.IsUnknown() {
		queryValue = data.Query.ValueString()
	}
	data.ID = hashDataSourceID("applications", data.BusinessGroupID.ValueString(), queryValue)
	data.Total = types.Int64Value(extractTotal(raw, len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

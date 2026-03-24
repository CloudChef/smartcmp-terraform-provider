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

var _ datasource.DataSource = &OSTemplatesDataSource{}

type OSTemplatesDataSource struct {
	client *client.Client
}

type OSTemplatesDataSourceModel struct {
	OSType           types.String `tfsdk:"os_type"`
	ResourceBundleID types.String `tfsdk:"resource_bundle_id"`
	ID               types.String `tfsdk:"id"`
	Total            types.Int64  `tfsdk:"total"`
	RawJSON          types.String `tfsdk:"raw_json"`
	Items            types.List   `tfsdk:"items"`
}

type OSTemplateItemModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	NameEn    types.String `tfsdk:"name_en"`
	OSVersion types.String `tfsdk:"os_version"`
	RawJSON   types.String `tfsdk:"raw_json"`
}

func NewOSTemplatesDataSource() datasource.DataSource {
	return &OSTemplatesDataSource{}
}

func (d *OSTemplatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_os_templates"
}

func (d *OSTemplatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List SmartCMP OS templates (logic templates) for a resource pool.",
		Attributes: map[string]schema.Attribute{
			"os_type":            schema.StringAttribute{Required: true},
			"resource_bundle_id": schema.StringAttribute{Required: true},
			"id":                 schema.StringAttribute{Computed: true},
			"total":              schema.Int64Attribute{Computed: true},
			"raw_json":           schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true},
						"name":       schema.StringAttribute{Computed: true},
						"name_en":    schema.StringAttribute{Computed: true},
						"os_version": schema.StringAttribute{Computed: true},
						"raw_json":   schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *OSTemplatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *OSTemplatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OSTemplatesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("expand", "")
	params.Set("osType", data.OSType.ValueString())
	params.Set("resourceBundleId", data.ResourceBundleID.ValueString())

	var raw any
	if err := d.client.GetJSON(ctx, "/logic-templates/search", params, &raw); err != nil {
		resp.Diagnostics.AddError("Read OS templates", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]OSTemplateItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, OSTemplateItemModel{
			ID:        types.StringValue(findFirstString(item, "id")),
			Name:      types.StringValue(findFirstString(item, "nameZh", "name", "templateName")),
			NameEn:    types.StringValue(findFirstString(item, "name")),
			OSVersion: types.StringValue(findFirstString(item, "osVersion", "version")),
			RawJSON:   jsonStringValue(item),
		})
	}

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":         types.StringType,
		"name":       types.StringType,
		"name_en":    types.StringType,
		"os_version": types.StringType,
		"raw_json":   types.StringType,
	}}
	listValue, diags := listValueFromStructs(ctx, objectType, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = hashDataSourceID("os_templates", data.OSType.ValueString(), data.ResourceBundleID.ValueString())
	data.Total = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

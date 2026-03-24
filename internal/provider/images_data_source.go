package provider

import (
	"context"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ datasource.DataSource = &ImagesDataSource{}

type ImagesDataSource struct {
	client *client.Client
}

type ImagesDataSourceModel struct {
	ResourceBundleID types.String `tfsdk:"resource_bundle_id"`
	LogicTemplateID  types.String `tfsdk:"logic_template_id"`
	CloudEntryTypeID types.String `tfsdk:"cloud_entry_type_id"`
	ID               types.String `tfsdk:"id"`
	Total            types.Int64  `tfsdk:"total"`
	RawJSON          types.String `tfsdk:"raw_json"`
	Items            types.List   `tfsdk:"items"`
}

type ImageItemModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	OSType  types.String `tfsdk:"os_type"`
	RawJSON types.String `tfsdk:"raw_json"`
}

func NewImagesDataSource() datasource.DataSource {
	return &ImagesDataSource{}
}

func (d *ImagesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_images"
}

func (d *ImagesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List SmartCMP images for a resource pool and logic template.",
		Attributes: map[string]schema.Attribute{
			"resource_bundle_id": schema.StringAttribute{Required: true},
			"logic_template_id":  schema.StringAttribute{Required: true},
			"cloud_entry_type_id": schema.StringAttribute{
				Required: true,
			},
			"id":       schema.StringAttribute{Computed: true},
			"total":    schema.Int64Attribute{Computed: true},
			"raw_json": schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":       schema.StringAttribute{Computed: true},
						"name":     schema.StringAttribute{Computed: true},
						"os_type":  schema.StringAttribute{Computed: true},
						"raw_json": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ImagesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ImagesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ImagesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var entryTypesRaw any
	params := url.Values{}
	params.Set("queryByCurrentTenant", "")
	if err := d.client.GetJSON(ctx, "/cloudentry-types/list_cloud_entry_types", params, &entryTypesRaw); err != nil {
		resp.Diagnostics.AddError("Validate cloud entry type", err.Error())
		return
	}
	for _, entryType := range extractItems(entryTypesRaw) {
		if findFirstString(entryType, "id") == data.CloudEntryTypeID.ValueString() && strings.EqualFold(findFirstString(entryType, "group"), "PUBLIC_CLOUD") {
			resp.Diagnostics.AddError("Images require a private cloud entry type", "smartcmp_images only supports PRIVATE_CLOUD entry types.")
			return
		}
	}

	cloudResourceType := data.CloudEntryTypeID.ValueString() + "::images"
	if strings.Contains(strings.ToLower(data.CloudEntryTypeID.ValueString()), "generic-cloud") {
		cloudResourceType = "yacmp:cloudentry:type:generic-cloud::images"
	}

	body := map[string]any{
		"cloudResourceType": cloudResourceType,
		"cloudEntryId":      nil,
		"businessGroupId":   nil,
		"queryProperties": map[string]any{
			"resourceBundleId":    data.ResourceBundleID.ValueString(),
			"logicTemplateId":     data.LogicTemplateID.ValueString(),
			"queryResourceBundle": false,
			"instanceType":        nil,
		},
		"limit": 500,
	}

	query := url.Values{}
	query.Set("action", "queryCloudResource")

	var raw any
	if err := d.client.PostJSON(ctx, "/cloudprovider", query, body, &raw); err != nil {
		resp.Diagnostics.AddError("Read images", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]ImageItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, ImageItemModel{
			ID:      types.StringValue(findFirstString(item, "id")),
			Name:    types.StringValue(findFirstString(item, "nameZh", "name", "imageName")),
			OSType:  types.StringValue(findFirstString(item, "osType", "osVersion", "version")),
			RawJSON: jsonStringValue(item),
		})
	}

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":       types.StringType,
		"name":     types.StringType,
		"os_type":  types.StringType,
		"raw_json": types.StringType,
	}}
	listValue, diags := listValueFromStructs(ctx, objectType, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = hashDataSourceID("images", data.ResourceBundleID.ValueString(), data.LogicTemplateID.ValueString(), data.CloudEntryTypeID.ValueString())
	data.Total = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

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

var _ datasource.DataSource = &ProfilesDataSource{}

type ProfilesDataSource struct {
	client *client.Client
}

type ProfilesDataSourceModel struct {
	Query            types.String `tfsdk:"query"`
	FlavorType       types.String `tfsdk:"flavor_type"`
	SpecType         types.String `tfsdk:"spec_type"`
	CatalogID        types.String `tfsdk:"catalog_id"`
	NodeTemplateName types.String `tfsdk:"node_template_name"`
	ResourceBundleID types.String `tfsdk:"resource_bundle_id"`
	CloudEntryTypeID types.String `tfsdk:"cloud_entry_type_id"`
	ProvisionScope   types.Bool   `tfsdk:"provision_scope"`
	ID               types.String `tfsdk:"id"`
	Total            types.Int64  `tfsdk:"total"`
	RawJSON          types.String `tfsdk:"raw_json"`
	Items            types.List   `tfsdk:"items"`
}

type ProfileItemModel struct {
	ID                  types.String  `tfsdk:"id"`
	Name                types.String  `tfsdk:"name"`
	NameEn              types.String  `tfsdk:"name_en"`
	Description         types.String  `tfsdk:"description"`
	SpecType            types.String  `tfsdk:"spec_type"`
	FlavorType          types.String  `tfsdk:"flavor_type"`
	ChangeWhenProvision types.Bool    `tfsdk:"change_when_provision"`
	MatchCPUAndMemory   types.Bool    `tfsdk:"match_cpu_and_memory"`
	CPU                 types.Int64   `tfsdk:"cpu"`
	MemoryGB            types.Int64   `tfsdk:"memory_gb"`
	Weight              types.Float64 `tfsdk:"weight"`
	RawJSON             types.String  `tfsdk:"raw_json"`
}

func NewProfilesDataSource() datasource.DataSource {
	return &ProfilesDataSource{}
}

func (d *ProfilesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profiles"
}

func (d *ProfilesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List SmartCMP provisioning profiles (ChargingResourceFlavor), including Linux VM T-shirt sizing profiles.",
		Attributes: map[string]schema.Attribute{
			"query":               schema.StringAttribute{Optional: true},
			"flavor_type":         schema.StringAttribute{Optional: true},
			"spec_type":           schema.StringAttribute{Optional: true},
			"catalog_id":          schema.StringAttribute{Optional: true},
			"node_template_name":  schema.StringAttribute{Optional: true},
			"resource_bundle_id":  schema.StringAttribute{Optional: true},
			"cloud_entry_type_id": schema.StringAttribute{Optional: true},
			"provision_scope": schema.BoolAttribute{
				Optional: true,
				Computed: true,
			},
			"id":       schema.StringAttribute{Computed: true},
			"total":    schema.Int64Attribute{Computed: true},
			"raw_json": schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                    schema.StringAttribute{Computed: true},
						"name":                  schema.StringAttribute{Computed: true},
						"name_en":               schema.StringAttribute{Computed: true},
						"description":           schema.StringAttribute{Computed: true},
						"spec_type":             schema.StringAttribute{Computed: true},
						"flavor_type":           schema.StringAttribute{Computed: true},
						"change_when_provision": schema.BoolAttribute{Computed: true},
						"match_cpu_and_memory":  schema.BoolAttribute{Computed: true},
						"cpu":                   schema.Int64Attribute{Computed: true},
						"memory_gb":             schema.Int64Attribute{Computed: true},
						"weight":                schema.Float64Attribute{Computed: true},
						"raw_json":              schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ProfilesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ProfilesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProfilesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	provisionScope := true
	if !data.ProvisionScope.IsNull() && !data.ProvisionScope.IsUnknown() {
		provisionScope = data.ProvisionScope.ValueBool()
	}

	query := url.Values{}
	query.Set("query", "")
	query.Set("page", "1")
	query.Set("size", "100")
	if !data.Query.IsNull() && !data.Query.IsUnknown() && data.Query.ValueString() != "" {
		query.Set("queryValue", data.Query.ValueString())
	}
	if !data.FlavorType.IsNull() && !data.FlavorType.IsUnknown() && data.FlavorType.ValueString() != "" {
		query.Set("flavorType", data.FlavorType.ValueString())
	}
	if !data.CatalogID.IsNull() && !data.CatalogID.IsUnknown() && data.CatalogID.ValueString() != "" {
		query.Set("catalogId", data.CatalogID.ValueString())
	}
	if !data.NodeTemplateName.IsNull() && !data.NodeTemplateName.IsUnknown() && data.NodeTemplateName.ValueString() != "" {
		query.Set("nodeTemplateName", data.NodeTemplateName.ValueString())
	}
	if !data.ResourceBundleID.IsNull() && !data.ResourceBundleID.IsUnknown() && data.ResourceBundleID.ValueString() != "" {
		query.Set("resourceBundleId", data.ResourceBundleID.ValueString())
	}
	if !data.CloudEntryTypeID.IsNull() && !data.CloudEntryTypeID.IsUnknown() && data.CloudEntryTypeID.ValueString() != "" {
		query.Set("cloudEntryTypeId", data.CloudEntryTypeID.ValueString())
	}
	if !data.SpecType.IsNull() && !data.SpecType.IsUnknown() && data.SpecType.ValueString() != "" {
		query.Set("specType", data.SpecType.ValueString())
	}

	path := "/flavors/provision"
	if !provisionScope {
		path = "/flavors"
	}

	var raw any
	if err := d.client.GetJSON(ctx, path, query, &raw); err != nil {
		resp.Diagnostics.AddError("Read profiles", err.Error())
		return
	}

	items := extractItems(raw)
	mapped := make([]ProfileItemModel, 0, len(items))
	for _, item := range items {
		cpu, memory := profileDimensions(item)
		mapped = append(mapped, ProfileItemModel{
			ID:                  types.StringValue(findFirstString(item, "id")),
			Name:                types.StringValue(findFirstString(item, "name")),
			NameEn:              types.StringValue(findFirstString(item, "nameEn")),
			Description:         types.StringValue(findFirstString(item, "descriptionEn", "description")),
			SpecType:            types.StringValue(findFirstString(item, "specType")),
			FlavorType:          types.StringValue(findFirstString(item, "flavorType")),
			ChangeWhenProvision: types.BoolValue(boolValue(item["changeWhenProvision"])),
			MatchCPUAndMemory:   types.BoolValue(boolValue(item["matchCpuAndMemory"])),
			CPU:                 types.Int64Value(cpu),
			MemoryGB:            types.Int64Value(memory),
			Weight:              types.Float64Value(numberValue(item["weight"])),
			RawJSON:             jsonStringValue(item),
		})
	}

	objectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":                    types.StringType,
		"name":                  types.StringType,
		"name_en":               types.StringType,
		"description":           types.StringType,
		"spec_type":             types.StringType,
		"flavor_type":           types.StringType,
		"change_when_provision": types.BoolType,
		"match_cpu_and_memory":  types.BoolType,
		"cpu":                   types.Int64Type,
		"memory_gb":             types.Int64Type,
		"weight":                types.Float64Type,
		"raw_json":              types.StringType,
	}}
	listValue, diags := listValueFromStructs(ctx, objectType, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ProvisionScope = types.BoolValue(provisionScope)
	data.ID = hashDataSourceID("profiles", path, data.FlavorType.ValueString(), data.CatalogID.ValueString(), data.NodeTemplateName.ValueString(), data.ResourceBundleID.ValueString(), data.CloudEntryTypeID.ValueString())
	data.Total = types.Int64Value(extractTotal(raw, len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func profileDimensions(item map[string]any) (int64, int64) {
	flavors := extractItems(item["flavors"])
	var cpu int64
	var memory int64
	for _, flavor := range flavors {
		switch strings.ToLower(findFirstString(flavor, "type")) {
		case "cpu":
			cpu = int64(numberValue(flavor["number"]))
		case "memory":
			memory = int64(numberValue(flavor["number"]))
		}
	}
	return cpu, memory
}

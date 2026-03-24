package provider

import (
	"context"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ datasource.DataSource = &ResourceActionsByIDsDataSource{}

type ResourceActionsByIDsDataSource struct {
	client *client.Client
}

type ResourceActionsByIDsDataSourceModel struct {
	ResourceCategory types.String `tfsdk:"resource_category"`
	ResourceIDs      types.List   `tfsdk:"resource_ids"`
	ID               types.String `tfsdk:"id"`
	TotalResources   types.Int64  `tfsdk:"total_resources"`
	RawJSON          types.String `tfsdk:"raw_json"`
	Items            types.List   `tfsdk:"items"`
}

type ResourceActionSetItemModel struct {
	ResourceID types.String `tfsdk:"resource_id"`
	Actions    types.List   `tfsdk:"actions"`
	RawJSON    types.String `tfsdk:"raw_json"`
}

func NewResourceActionsByIDsDataSource() datasource.DataSource {
	return &ResourceActionsByIDsDataSource{}
}

func (d *ResourceActionsByIDsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_actions_by_ids"
}

func (d *ResourceActionsByIDsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List day-two actions for each resource in a batch query, preserving the per-resource action sets returned by SmartCMP.",
		Attributes: map[string]schema.Attribute{
			"resource_category": schema.StringAttribute{
				MarkdownDescription: "Resource category path segment such as `iaas.machine`, `resource.software`, or `-1`.",
				Required:            true,
			},
			"resource_ids": schema.ListAttribute{
				MarkdownDescription: "Resource identifiers to query through the SmartCMP batch resource-actions endpoint.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"id":              schema.StringAttribute{Computed: true},
			"total_resources": schema.Int64Attribute{Computed: true},
			"raw_json":        schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource_id": schema.StringAttribute{Computed: true},
						"raw_json":    schema.StringAttribute{Computed: true},
						"actions": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: operationActionSchemaAttributes(),
							},
						},
					},
				},
			},
		},
	}
}

func (d *ResourceActionsByIDsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ResourceActionsByIDsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ResourceActionsByIDsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var resourceIDs []string
	resp.Diagnostics.Append(data.ResourceIDs.ElementsAs(ctx, &resourceIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(resourceIDs) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("resource_ids"), "Missing resource_ids", "Set at least one resource ID for batch action discovery.")
		return
	}

	body := map[string]any{"ids": resourceIDs}
	var raw map[string]any
	if err := d.client.PostJSON(ctx, "/nodes/"+url.PathEscape(data.ResourceCategory.ValueString())+"/batch/resource-actions", nil, body, &raw); err != nil {
		resp.Diagnostics.AddError("Read resource actions by IDs", err.Error())
		return
	}

	actionObjectType := types.ObjectType{AttrTypes: operationActionAttrTypes()}
	items := make([]ResourceActionSetItemModel, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		resourceID = strings.TrimSpace(resourceID)
		if resourceID == "" {
			continue
		}

		rawActions := extractItems(raw[resourceID])
		mappedActions := make([]OperationActionItemModel, 0, len(rawActions))
		for _, item := range rawActions {
			mappedActions = append(mappedActions, mapOperationActionItem(item))
		}

		actionsList, diags := listValueFromStructs(ctx, actionObjectType, mappedActions)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		items = append(items, ResourceActionSetItemModel{
			ResourceID: types.StringValue(resourceID),
			Actions:    actionsList,
			RawJSON:    jsonStringValue(raw[resourceID]),
		})
	}

	resourceItemType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"resource_id": types.StringType,
		"actions":     types.ListType{ElemType: actionObjectType},
		"raw_json":    types.StringType,
	}}
	itemsList, diags := listValueFromStructs(ctx, resourceItemType, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = hashDataSourceID("resource_actions_by_ids", data.ResourceCategory.ValueString(), strings.Join(resourceIDs, ","))
	data.TotalResources = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = itemsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

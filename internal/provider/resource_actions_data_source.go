package provider

import (
	"context"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ datasource.DataSource = &ResourceActionsDataSource{}

type ResourceActionsDataSource struct {
	client *client.Client
}

type ResourceActionsDataSourceModel struct {
	ResourceCategory types.String `tfsdk:"resource_category"`
	ResourceID       types.String `tfsdk:"resource_id"`
	ResourceIDs      types.List   `tfsdk:"resource_ids"`
	TaskType         types.String `tfsdk:"task_type"`
	ID               types.String `tfsdk:"id"`
	Total            types.Int64  `tfsdk:"total"`
	RawJSON          types.String `tfsdk:"raw_json"`
	Items            types.List   `tfsdk:"items"`
}

func NewResourceActionsDataSource() datasource.DataSource {
	return &ResourceActionsDataSource{}
}

func (d *ResourceActionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_actions"
}

func (d *ResourceActionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List day-two actions for a SmartCMP resource, or common actions shared by multiple resources in the same category.",
		Attributes: map[string]schema.Attribute{
			"resource_category": schema.StringAttribute{
				MarkdownDescription: "Resource category path segment such as `iaas.machine`, `resource.software`, or `-1`.",
				Required:            true,
			},
			"resource_id": schema.StringAttribute{
				MarkdownDescription: "Single resource identifier. Use this for exact per-resource action discovery.",
				Optional:            true,
			},
			"resource_ids": schema.ListAttribute{
				MarkdownDescription: "Multiple resource identifiers. When set, the datasource returns the common actions shared by all listed resources.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"task_type": schema.StringAttribute{
				MarkdownDescription: "Optional task type passed to the single-resource action endpoint.",
				Optional:            true,
			},
			"id":       schema.StringAttribute{Computed: true},
			"total":    schema.Int64Attribute{Computed: true},
			"raw_json": schema.StringAttribute{Computed: true},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: operationActionSchemaAttributes(),
				},
			},
		},
	}
}

func (d *ResourceActionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ResourceActionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ResourceActionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceID := ""
	if !data.ResourceID.IsNull() && !data.ResourceID.IsUnknown() {
		resourceID = strings.TrimSpace(data.ResourceID.ValueString())
	}

	resourceIDs := []string{}
	if !data.ResourceIDs.IsNull() && !data.ResourceIDs.IsUnknown() {
		resp.Diagnostics.Append(data.ResourceIDs.ElementsAs(ctx, &resourceIDs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if resourceID != "" && len(resourceIDs) > 0 {
		resp.Diagnostics.AddAttributeError(path.Root("resource_id"), "Conflicting resource selectors", "Set either resource_id or resource_ids, not both.")
		return
	}
	if resourceID == "" && len(resourceIDs) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("resource_id"), "Missing resource selector", "Set resource_id for a single resource or resource_ids for common batch actions.")
		return
	}
	if len(resourceIDs) > 0 && !data.TaskType.IsNull() && !data.TaskType.IsUnknown() && strings.TrimSpace(data.TaskType.ValueString()) != "" {
		resp.Diagnostics.AddAttributeError(path.Root("task_type"), "Unsupported task_type for batch lookup", "task_type is only supported with resource_id.")
		return
	}

	var raw any
	if resourceID != "" {
		params := url.Values{}
		if !data.TaskType.IsNull() && !data.TaskType.IsUnknown() && strings.TrimSpace(data.TaskType.ValueString()) != "" {
			params.Set("taskType", data.TaskType.ValueString())
		}

		if err := d.client.GetJSON(ctx, "/nodes/"+url.PathEscape(data.ResourceCategory.ValueString())+"/"+url.PathEscape(resourceID)+"/resource-actions", params, &raw); err != nil {
			resp.Diagnostics.AddError("Read resource actions", err.Error())
			return
		}
	} else {
		body := map[string]any{"ids": resourceIDs}
		if err := d.client.PostJSON(ctx, "/nodes/"+url.PathEscape(data.ResourceCategory.ValueString())+"/resource-actions", nil, body, &raw); err != nil {
			resp.Diagnostics.AddError("Read resource actions", err.Error())
			return
		}
	}

	items := extractItems(raw)
	mapped := make([]OperationActionItemModel, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, mapOperationActionItem(item))
	}

	listValue, diags := listValueFromStructs(ctx, types.ObjectType{AttrTypes: operationActionAttrTypes()}, mapped)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = hashDataSourceID("resource_actions", data.ResourceCategory.ValueString(), resourceID, strings.Join(resourceIDs, ","), data.TaskType.ValueString())
	data.Total = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

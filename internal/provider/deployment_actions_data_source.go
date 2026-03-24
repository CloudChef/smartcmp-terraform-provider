package provider

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ datasource.DataSource = &DeploymentActionsDataSource{}

type DeploymentActionsDataSource struct {
	client *client.Client
}

type DeploymentActionsDataSourceModel struct {
	DeploymentID types.String `tfsdk:"deployment_id"`
	ID           types.String `tfsdk:"id"`
	Total        types.Int64  `tfsdk:"total"`
	RawJSON      types.String `tfsdk:"raw_json"`
	Items        types.List   `tfsdk:"items"`
}

func NewDeploymentActionsDataSource() datasource.DataSource {
	return &DeploymentActionsDataSource{}
}

func (d *DeploymentActionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_actions"
}

func (d *DeploymentActionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List day-two actions available for a specific SmartCMP deployment.",
		Attributes: map[string]schema.Attribute{
			"deployment_id": schema.StringAttribute{
				MarkdownDescription: "Deployment identifier.",
				Required:            true,
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

func (d *DeploymentActionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *DeploymentActionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DeploymentActionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var raw any
	if err := d.client.GetJSON(ctx, "/deployments/"+url.PathEscape(data.DeploymentID.ValueString())+"/deployment-actions", nil, &raw); err != nil {
		resp.Diagnostics.AddError("Read deployment actions", err.Error())
		return
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

	data.ID = hashDataSourceID("deployment_actions", data.DeploymentID.ValueString())
	data.Total = types.Int64Value(int64(len(items)))
	data.RawJSON = jsonStringValue(raw)
	data.Items = listValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

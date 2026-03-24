package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	resourcetimeouts "github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var (
	_ resource.Resource                = &ServiceRequestResource{}
	_ resource.ResourceWithImportState = &ServiceRequestResource{}
	_ resource.Resource                = &ResourceOperationResource{}
	_ resource.ResourceWithImportState = &ResourceOperationResource{}
)

type ServiceRequestResource struct {
	client *client.Client
}

type ResourceOperationResource struct {
	client *client.Client
}

type ServiceRequestResourceModel struct {
	ID                     types.String           `tfsdk:"id"`
	CatalogID              types.String           `tfsdk:"catalog_id"`
	BusinessGroupID        types.String           `tfsdk:"business_group_id"`
	Name                   types.String           `tfsdk:"name"`
	Description            types.String           `tfsdk:"description"`
	ProjectID              types.String           `tfsdk:"project_id"`
	RequestUserID          types.String           `tfsdk:"request_user_id"`
	Count                  types.Int64            `tfsdk:"request_count"`
	AddToInventory         types.Bool             `tfsdk:"add_to_inventory"`
	ResourcePoolID         types.String           `tfsdk:"resource_pool_id"`
	ResourcePoolTags       types.List             `tfsdk:"resource_pool_tags"`
	ResourcePoolParamsJSON types.String           `tfsdk:"resource_pool_params_json"`
	ResourceSpecs          types.List             `tfsdk:"resource_specs"`
	RequestBodyJSON        types.String           `tfsdk:"request_body_json"`
	WaitForTerminalState   types.Bool             `tfsdk:"wait_for_terminal_state"`
	Timeouts               resourcetimeouts.Value `tfsdk:"timeouts"`
	State                  types.String           `tfsdk:"state"`
	ErrorMessage           types.String           `tfsdk:"error_message"`
	CompletedAt            types.String           `tfsdk:"completed_at"`
	InventoryID            types.String           `tfsdk:"inventory_id"`
	ObjectID               types.String           `tfsdk:"object_id"`
	ObjectType             types.String           `tfsdk:"object_type"`
	DeploymentIDs          types.List             `tfsdk:"deployment_ids"`
}

type ResourceOperationResourceModel struct {
	ID                   types.String           `tfsdk:"id"`
	TargetKind           types.String           `tfsdk:"target_kind"`
	TargetID             types.String           `tfsdk:"target_id"`
	Operation            types.String           `tfsdk:"operation"`
	Comment              types.String           `tfsdk:"comment"`
	ScheduledTime        types.String           `tfsdk:"scheduled_time"`
	ParametersJSON       types.String           `tfsdk:"parameters_json"`
	WaitForTerminalState types.Bool             `tfsdk:"wait_for_terminal_state"`
	Timeouts             resourcetimeouts.Value `tfsdk:"timeouts"`
	TaskID               types.String           `tfsdk:"task_id"`
	TaskState            types.String           `tfsdk:"task_state"`
	TaskSubStage         types.String           `tfsdk:"task_sub_stage"`
	ResultMessage        types.String           `tfsdk:"result_message"`
	GenericRequestID     types.String           `tfsdk:"generic_request_id"`
	ResourceIDs          types.List             `tfsdk:"resource_ids"`
	DeploymentID         types.String           `tfsdk:"deployment_id"`
}

func NewServiceRequestResource() resource.Resource {
	return &ServiceRequestResource{}
}

func NewResourceOperationResource() resource.Resource {
	return &ResourceOperationResource{}
}

func (r *ServiceRequestResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_request"
}

func (r *ResourceOperationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_operation"
}

func (r *ServiceRequestResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Submit and track a SmartCMP service request.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"catalog_id": schema.StringAttribute{
				MarkdownDescription: "SmartCMP catalog ID.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"business_group_id": schema.StringAttribute{
				MarkdownDescription: "Business group ID that owns the request.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Request display name.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional request description. It is mirrored into genericRequest.description when omitted from JSON.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Optional project/group ID. This is sent as projectId and groupId for compatibility.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"request_user_id": schema.StringAttribute{
				MarkdownDescription: "Optional SmartCMP requester user ID. When omitted, SmartCMP may infer the authenticated user.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"request_count": schema.Int64Attribute{
				MarkdownDescription: "Optional request count. Defaults to 1.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"add_to_inventory": schema.BoolAttribute{
				MarkdownDescription: "Whether the request should be added to inventory instead of submitted immediately.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"resource_pool_id": schema.StringAttribute{
				MarkdownDescription: "Optional default resource bundle ID applied to the request payload and resource specs.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_pool_tags": schema.ListAttribute{
				MarkdownDescription: "Optional default resource bundle tags applied when resource specs do not set resourceBundleTags.",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"resource_pool_params_json": schema.StringAttribute{
				MarkdownDescription: "Optional default JSON object applied as resourceBundleParams when resource specs do not set it.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"request_body_json": schema.StringAttribute{
				MarkdownDescription: "Optional raw JSON object merged into the submit payload before typed attributes are overlaid.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_specs": schema.ListNestedAttribute{
				MarkdownDescription: "Optional typed wrapper for SmartCMP resourceSpecs. Use spec_json for service-specific fields.",
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"node": schema.StringAttribute{
							Optional: true,
						},
						"type": schema.StringAttribute{
							Optional: true,
						},
						"resource_pool_id": schema.StringAttribute{
							Optional: true,
						},
						"resource_pool_tags": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
						"resource_pool_params_json": schema.StringAttribute{
							Optional: true,
						},
						"spec_json": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"wait_for_terminal_state": schema.BoolAttribute{
				MarkdownDescription: "Whether create should wait until the request reaches a terminal state.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"timeouts": resourcetimeouts.Attributes(ctx, resourcetimeouts.Opts{
				Create: true,
				Read:   true,
				Delete: true,
			}),
			"state":         schema.StringAttribute{Computed: true},
			"error_message": schema.StringAttribute{Computed: true},
			"completed_at":  schema.StringAttribute{Computed: true},
			"inventory_id":  schema.StringAttribute{Computed: true},
			"object_id":     schema.StringAttribute{Computed: true},
			"object_type":   schema.StringAttribute{Computed: true},
			"deployment_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *ResourceOperationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Trigger and track a SmartCMP resource or deployment operation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"target_kind": schema.StringAttribute{
				MarkdownDescription: "Operation target kind: resource or deployment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"target_id": schema.StringAttribute{
				MarkdownDescription: "Resource ID or deployment ID to operate on.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"operation": schema.StringAttribute{
				MarkdownDescription: "Operation identifier or name expected by SmartCMP.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comment": schema.StringAttribute{
				MarkdownDescription: "Optional operation comment.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scheduled_time": schema.StringAttribute{
				MarkdownDescription: "Optional scheduled execution time forwarded to scheduledTaskMetadataRequest.scheduledTime.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parameters_json": schema.StringAttribute{
				MarkdownDescription: "Optional raw JSON object. It can be either the full SmartCMP request shape or just the operation parameters map.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"wait_for_terminal_state": schema.BoolAttribute{
				MarkdownDescription: "Whether create should wait until the task reaches a terminal state.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"timeouts": resourcetimeouts.Attributes(ctx, resourcetimeouts.Opts{
				Create: true,
				Read:   true,
				Delete: true,
			}),
			"task_id":            schema.StringAttribute{Computed: true},
			"task_state":         schema.StringAttribute{Computed: true},
			"task_sub_stage":     schema.StringAttribute{Computed: true},
			"result_message":     schema.StringAttribute{Computed: true},
			"generic_request_id": schema.StringAttribute{Computed: true},
			"deployment_id":      schema.StringAttribute{Computed: true},
			"resource_ids":       schema.ListAttribute{Computed: true, ElementType: types.StringType},
		},
	}
}

func (r *ServiceRequestResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *ResourceOperationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *ServiceRequestResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data ServiceRequestResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := data.Timeouts.Create(ctx, defaultResourceCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, err := buildServiceRequestPayload(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Build SmartCMP service request payload", err.Error())
		return
	}

	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	var raw any
	if err := r.client.PostJSON(createCtx, "/generic-request/submit", nil, payload, &raw); err != nil {
		resp.Diagnostics.AddError("Create SmartCMP service request", err.Error())
		return
	}

	requestRaw, err := extractSingleRequest(raw)
	if err != nil {
		resp.Diagnostics.AddError("Create SmartCMP service request", err.Error())
		return
	}

	applyServiceRequestRaw(&data, requestRaw)
	if warning, diags := populateServiceRequestDeploymentIDs(createCtx, r.client, &data); warning != "" {
		resp.Diagnostics.AddWarning("Lookup related deployments", warning)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.Diagnostics.Append(diags...)
	}

	if data.WaitForTerminalState.ValueBool() && !data.ID.IsNull() && !data.ID.IsUnknown() {
		latest, err := waitForRequestTerminal(createCtx, r.client, data.ID.ValueString())
		if err != nil && latest != nil {
			applyServiceRequestRaw(&data, latest)
			if warning, diags := populateServiceRequestDeploymentIDs(createCtx, r.client, &data); warning != "" {
				resp.Diagnostics.AddWarning("Lookup related deployments", warning)
				resp.Diagnostics.Append(diags...)
			} else {
				resp.Diagnostics.Append(diags...)
			}
		}
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				resp.Diagnostics.AddWarning("Timed out waiting for terminal request state", "The SmartCMP request was created and is tracked in Terraform state with the latest observed status.")
			} else {
				resp.Diagnostics.AddError("Wait for terminal request state", err.Error())
				return
			}
		} else {
			applyServiceRequestRaw(&data, latest)
			if warning, diags := populateServiceRequestDeploymentIDs(createCtx, r.client, &data); warning != "" {
				resp.Diagnostics.AddWarning("Lookup related deployments", warning)
				resp.Diagnostics.Append(diags...)
			} else {
				resp.Diagnostics.Append(diags...)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceRequestResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data ServiceRequestResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, defaultResourceReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	raw, err := fetchServiceRequest(readCtx, r.client, data.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Read SmartCMP service request", err.Error())
		return
	}

	applyServiceRequestRaw(&data, raw)
	if warning, diags := populateServiceRequestDeploymentIDs(readCtx, r.client, &data); warning != "" {
		resp.Diagnostics.AddWarning("Lookup related deployments", warning)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.Diagnostics.Append(diags...)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceRequestResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var plan ServiceRequestResourceModel
	var state ServiceRequestResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, defaultResourceReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	raw, err := fetchServiceRequest(readCtx, r.client, state.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Refresh SmartCMP service request", err.Error())
		return
	}

	plan.ID = state.ID
	applyServiceRequestRaw(&plan, raw)
	if warning, diags := populateServiceRequestDeploymentIDs(readCtx, r.client, &plan); warning != "" {
		resp.Diagnostics.AddWarning("Lookup related deployments", warning)
		resp.Diagnostics.Append(diags...)
	} else {
		resp.Diagnostics.Append(diags...)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServiceRequestResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data ServiceRequestResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := data.Timeouts.Delete(ctx, defaultResourceDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	raw, err := fetchServiceRequest(deleteCtx, r.client, data.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			return
		}

		resp.Diagnostics.AddError("Delete SmartCMP service request", err.Error())
		return
	}

	if requestIsTerminal(findFirstString(raw, "state")) {
		return
	}

	if err := r.client.PostJSON(deleteCtx, "/generic-request/"+url.PathEscape(data.ID.ValueString())+"/cancel", nil, nil, nil); err != nil {
		resp.Diagnostics.AddError("Cancel SmartCMP service request", err.Error())
		return
	}

	if data.WaitForTerminalState.ValueBool() {
		if _, err := waitForRequestTerminal(deleteCtx, r.client, data.ID.ValueString()); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			resp.Diagnostics.AddError("Wait for request cancellation", err.Error())
			return
		} else if errors.Is(err, context.DeadlineExceeded) {
			resp.Diagnostics.AddWarning("Timed out waiting for request cancellation", "The cancellation request was sent successfully. Terraform will forget this request while SmartCMP finishes processing it.")
		}
	}
}

func (r *ServiceRequestResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ResourceOperationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data ResourceOperationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateTargetKind(data.TargetKind.ValueString()); err != nil {
		resp.Diagnostics.AddError("Invalid target_kind", err.Error())
		return
	}

	createTimeout, diags := data.Timeouts.Create(ctx, defaultResourceCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	parameters, err := parseOptionalJSONObject(data.ParametersJSON, "parameters_json")
	if err != nil {
		resp.Diagnostics.AddError("Invalid parameters_json", err.Error())
		return
	}

	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	var taskRaw map[string]any
	switch data.TargetKind.ValueString() {
	case "deployment":
		payload := cloneMap(parameters)
		if !isFullDeploymentOperationRequest(payload) {
			payload = map[string]any{
				"params": payload,
			}
		}

		payload["operationName"] = data.Operation.ValueString()
		if !data.Comment.IsNull() && !data.Comment.IsUnknown() && data.Comment.ValueString() != "" {
			payload["comment"] = data.Comment.ValueString()
		}
		if !data.ScheduledTime.IsNull() && !data.ScheduledTime.IsUnknown() && data.ScheduledTime.ValueString() != "" {
			setNestedAny(payload, data.ScheduledTime.ValueString(), "scheduledTaskMetadataRequest", "scheduledTime")
		}

		if err := r.client.PostJSON(createCtx, "/deployments/"+url.PathEscape(data.TargetID.ValueString())+"/day2-op", nil, payload, &taskRaw); err != nil {
			resp.Diagnostics.AddError("Create SmartCMP deployment operation", err.Error())
			return
		}

	case "resource":
		payload := cloneMap(parameters)
		if !isFullResourceOperationRequest(payload) {
			payload = map[string]any{
				"executeParameters": payload,
			}
		}

		payload["operationId"] = data.Operation.ValueString()
		payload["resourceIds"] = []string{data.TargetID.ValueString()}
		if !data.Comment.IsNull() && !data.Comment.IsUnknown() && data.Comment.ValueString() != "" {
			payload["comment"] = data.Comment.ValueString()
		}
		if !data.ScheduledTime.IsNull() && !data.ScheduledTime.IsUnknown() && data.ScheduledTime.ValueString() != "" {
			setNestedAny(payload, data.ScheduledTime.ValueString(), "scheduledTaskMetadataRequest", "scheduledTime")
		}

		var batchRaw map[string]any
		if err := r.client.PostJSON(createCtx, "/nodes/resource-operations", nil, payload, &batchRaw); err != nil {
			resp.Diagnostics.AddError("Create SmartCMP resource operation", err.Error())
			return
		}

		taskRaw, err = extractTaskFromBatchResponse(batchRaw, data.TargetID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Create SmartCMP resource operation", err.Error())
			return
		}
	default:
		resp.Diagnostics.AddError("Invalid target_kind", fmt.Sprintf("unsupported target kind %q", data.TargetKind.ValueString()))
		return
	}

	resp.Diagnostics.Append(applyTaskRaw(ctx, &data, taskRaw)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.WaitForTerminalState.ValueBool() && !data.ID.IsNull() && !data.ID.IsUnknown() {
		latest, err := waitForTaskTerminal(createCtx, r.client, data.ID.ValueString())
		if err != nil && latest != nil {
			resp.Diagnostics.Append(applyTaskRaw(ctx, &data, latest)...)
		}
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				resp.Diagnostics.AddWarning("Timed out waiting for terminal task state", "The SmartCMP task was created and is tracked in Terraform state with the latest observed status.")
			} else {
				resp.Diagnostics.AddError("Wait for terminal task state", err.Error())
				return
			}
		} else {
			resp.Diagnostics.Append(applyTaskRaw(ctx, &data, latest)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceOperationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data ResourceOperationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, defaultResourceReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	raw, err := fetchTask(readCtx, r.client, data.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Read SmartCMP task", err.Error())
		return
	}

	resp.Diagnostics.Append(applyTaskRaw(ctx, &data, raw)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceOperationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var plan ResourceOperationResourceModel
	var state ResourceOperationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, defaultResourceReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	raw, err := fetchTask(readCtx, r.client, state.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Refresh SmartCMP task", err.Error())
		return
	}

	plan.ID = state.ID
	plan.TaskID = state.TaskID
	resp.Diagnostics.Append(applyTaskRaw(ctx, &plan, raw)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ResourceOperationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data ResourceOperationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := data.Timeouts.Delete(ctx, defaultResourceDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	raw, err := fetchTask(deleteCtx, r.client, data.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			return
		}

		resp.Diagnostics.AddError("Delete SmartCMP task", err.Error())
		return
	}

	if taskIsTerminal(findFirstString(raw, "state")) {
		return
	}

	var cancelled map[string]any
	if err := r.client.PutJSON(deleteCtx, "/tasks/"+url.PathEscape(data.ID.ValueString())+"/cancel", nil, nil, &cancelled); err != nil {
		resp.Diagnostics.AddError("Cancel SmartCMP task", err.Error())
		return
	}

	if data.WaitForTerminalState.ValueBool() {
		if _, err := waitForTaskTerminal(deleteCtx, r.client, data.ID.ValueString()); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			resp.Diagnostics.AddError("Wait for task cancellation", err.Error())
			return
		} else if errors.Is(err, context.DeadlineExceeded) {
			resp.Diagnostics.AddWarning("Timed out waiting for task cancellation", "The cancellation request was sent successfully. Terraform will forget this task while SmartCMP finishes processing it.")
		}
	}
}

func (r *ResourceOperationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func validateTargetKind(targetKind string) error {
	switch targetKind {
	case "deployment", "resource":
		return nil
	default:
		return fmt.Errorf("target_kind must be either %q or %q", "deployment", "resource")
	}
}

package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	resourcetimeouts "github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ resource.Resource = &VirtualMachineResource{}

type VirtualMachineResource struct {
	client *client.Client
}

type VirtualMachineResourceModel struct {
	ID                 types.String           `tfsdk:"id"`
	CatalogID          types.String           `tfsdk:"catalog_id"`
	BusinessGroupID    types.String           `tfsdk:"business_group_id"`
	Name               types.String           `tfsdk:"name"`
	Description        types.String           `tfsdk:"description"`
	RequestUserID      types.String           `tfsdk:"request_user_id"`
	ResourcePoolID     types.String           `tfsdk:"resource_pool_id"`
	InstanceType       types.String           `tfsdk:"instance_type"`
	ComputeProfileID   types.String           `tfsdk:"compute_profile_id"`
	LogicTemplateID    types.String           `tfsdk:"logic_template_id"`
	PhysicalTemplateID types.String           `tfsdk:"physical_template_id"`
	TemplateID         types.String           `tfsdk:"template_id"`
	CredentialUser     types.String           `tfsdk:"credential_user"`
	CredentialPassword types.String           `tfsdk:"credential_password"`
	NetworkID          types.String           `tfsdk:"network_id"`
	SubnetID           types.String           `tfsdk:"subnet_id"`
	SecurityGroupIDs   types.List             `tfsdk:"security_group_ids"`
	SystemDisk         types.Object           `tfsdk:"system_disk"`
	DataDisks          types.List             `tfsdk:"data_disks"`
	StartAfterResize   types.Bool             `tfsdk:"start_after_resize"`
	PowerState         types.String           `tfsdk:"power_state"`
	CPU                types.Int64            `tfsdk:"cpu"`
	MemoryGB           types.Float64          `tfsdk:"memory_gb"`
	Timeouts           resourcetimeouts.Value `tfsdk:"timeouts"`
	RequestID          types.String           `tfsdk:"request_id"`
	DeploymentID       types.String           `tfsdk:"deployment_id"`
	ResourceID         types.String           `tfsdk:"resource_id"`
	Status             types.String           `tfsdk:"status"`
	InstanceTypeActual types.String           `tfsdk:"instance_type_actual"`
}

type virtualMachineDiskModel struct {
	Name         types.String `tfsdk:"name"`
	Size         types.Int64  `tfsdk:"size"`
	IsSystemDisk types.Bool   `tfsdk:"is_system_disk"`
	VolumeType   types.String `tfsdk:"volume_type"`
	DiskPolicy   types.String `tfsdk:"disk_policy"`
	DiskTags     types.List   `tfsdk:"disk_tags"`
}

type virtualMachineSystemDiskModel struct {
	Size         types.Int64  `tfsdk:"size"`
	IsSystemDisk types.Bool   `tfsdk:"is_system_disk"`
	VolumeType   types.String `tfsdk:"volume_type"`
	DiskPolicy   types.String `tfsdk:"disk_policy"`
	DiskTags     types.List   `tfsdk:"disk_tags"`
}

func NewVirtualMachineResource() resource.Resource {
	return &VirtualMachineResource{}
}

func (r *VirtualMachineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_machine"
}

func (r *VirtualMachineResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provision and manage a Terraform-owned SmartCMP virtual machine.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"catalog_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"business_group_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"request_user_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_pool_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance_type": schema.StringAttribute{
				Optional: true,
			},
			"compute_profile_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"logic_template_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"physical_template_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential_user": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential_password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"network_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subnet_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"security_group_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"system_disk": schema.SingleNestedAttribute{
				Optional:   true,
				Attributes: virtualMachineDiskAttributes(false),
			},
			"data_disks": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: virtualMachineDiskAttributes(true),
				},
			},
			"start_after_resize": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"power_state": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"cpu": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"memory_gb": schema.Float64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.UseStateForUnknown(),
				},
			},
			"timeouts": resourcetimeouts.Attributes(ctx, resourcetimeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
			"request_id":           schema.StringAttribute{Computed: true},
			"deployment_id":        schema.StringAttribute{Computed: true},
			"resource_id":          schema.StringAttribute{Computed: true},
			"status":               schema.StringAttribute{Computed: true},
			"instance_type_actual": schema.StringAttribute{Computed: true},
		},
	}
}

func virtualMachineDiskAttributes(includeName bool) map[string]schema.Attribute {
	attrs := map[string]schema.Attribute{
		"size": schema.Int64Attribute{
			Optional: true,
		},
		"is_system_disk": schema.BoolAttribute{
			Optional: true,
		},
		"volume_type": schema.StringAttribute{
			Optional: true,
		},
		"disk_policy": schema.StringAttribute{
			Optional: true,
		},
		"disk_tags": schema.ListAttribute{
			Optional:    true,
			ElementType: types.StringType,
		},
	}
	if includeName {
		attrs["name"] = schema.StringAttribute{Optional: true}
	}
	return attrs
}

func (r *VirtualMachineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *VirtualMachineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data VirtualMachineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	desiredPowerState, hasDesiredPowerState, err := resolveDesiredPowerState(data.PowerState)
	if err != nil {
		resp.Diagnostics.AddError("Invalid SmartCMP virtual machine power_state", err.Error())
		return
	}

	createTimeout, diags := data.Timeouts.Create(ctx, defaultResourceCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, err := buildVirtualMachineRequestPayload(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Build SmartCMP virtual machine request", err.Error())
		return
	}

	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	var submitRaw any
	if err := r.client.PostJSON(createCtx, "/generic-request/submit", nil, payload, &submitRaw); err != nil {
		resp.Diagnostics.AddError("Create SmartCMP virtual machine", err.Error())
		return
	}

	requestRaw, err := extractSingleRequest(submitRaw)
	if err != nil {
		resp.Diagnostics.AddError("Create SmartCMP virtual machine", err.Error())
		return
	}
	data.RequestID = nullableString(findFirstString(requestRaw, "id"))

	latestRequest, err := waitForRequestTerminal(createCtx, r.client, data.RequestID.ValueString())
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			resp.Diagnostics.AddError("Create SmartCMP virtual machine", "timed out waiting for the SmartCMP request to finish")
		} else {
			resp.Diagnostics.AddError("Create SmartCMP virtual machine", err.Error())
		}
		return
	}

	requestState := findFirstString(latestRequest, "state")
	if !strings.EqualFold(requestState, "FINISHED") {
		resp.Diagnostics.AddError("Create SmartCMP virtual machine", fmt.Sprintf("request %s finished in state %s: %s", data.RequestID.ValueString(), requestState, findFirstString(latestRequest, "errMsg", "errorMessage", "message")))
		return
	}

	deploymentIDs, err := lookupDeploymentIDs(createCtx, r.client, data.RequestID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Resolve SmartCMP deployment", err.Error())
		return
	}
	requestCreatedAt := int64(numberValue(latestRequest["createdDate"]))

	var nodeRaw map[string]any
	if len(deploymentIDs) > 0 {
		data.DeploymentID = types.StringValue(deploymentIDs[0])

		nodeRaw, err = waitForVirtualMachineNode(createCtx, r.client, data.DeploymentID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Resolve SmartCMP virtual machine resource", err.Error())
			return
		}
	} else {
		nodeRaw, err = waitForVirtualMachineNodeByName(createCtx, r.client, data.Name.ValueString(), requestCreatedAt)
		if err != nil {
			resp.Diagnostics.AddError("Resolve SmartCMP deployment", "request finished successfully but no deployment was linked to the request")
			return
		}
	}
	applyVirtualMachineNodeRaw(&data, nodeRaw)
	if hasDesiredPowerState {
		// Catalog requests typically return a started VM, so a configured power_state
		// may still require a follow-up day-two operation after provisioning completes.
		resourceID := data.ID.ValueString()
		if resourceID == "" {
			resourceID = data.ResourceID.ValueString()
		}
		actualPowerState := stringValueFromType(data.PowerState)
		actualPowerState, err = reconcileVirtualMachinePowerState(createCtx, r.client, resourceID, actualPowerState, desiredPowerState)
		if err != nil {
			resp.Diagnostics.AddError("Reconcile SmartCMP virtual machine power state", err.Error())
			return
		}
		if actualPowerState != stringValueFromType(data.PowerState) {
			nodeRaw, err = r.readOrResolveVirtualMachine(createCtx, &data)
			if err != nil {
				resp.Diagnostics.AddError("Read SmartCMP virtual machine", err.Error())
				return
			}
			applyVirtualMachineNodeRaw(&data, nodeRaw)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtualMachineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data VirtualMachineResourceModel
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

	nodeRaw, err := r.readOrResolveVirtualMachine(readCtx, &data)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read SmartCMP virtual machine", err.Error())
		return
	}

	applyVirtualMachineNodeRaw(&data, nodeRaw)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VirtualMachineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var plan VirtualMachineResourceModel
	var state VirtualMachineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	desiredPowerState, hasDesiredPowerState, err := resolveDesiredPowerState(plan.PowerState)
	if err != nil {
		resp.Diagnostics.AddError("Invalid SmartCMP virtual machine power_state", err.Error())
		return
	}

	updateTimeout, diags := state.Timeouts.Update(ctx, defaultResourceCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateCtx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	nodeRaw, err := r.readOrResolveVirtualMachine(updateCtx, &state)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Refresh SmartCMP virtual machine", err.Error())
		return
	}

	plan.ID = state.ID
	plan.RequestID = state.RequestID
	plan.DeploymentID = state.DeploymentID
	plan.ResourceID = state.ResourceID

	currentInstanceType := findFirstString(nodeRaw, "instanceType", "flavorId")
	currentPowerState := normalizeVirtualMachinePowerState(findFirstString(nodeRaw, "status"))
	desiredInstanceType := stringValueFromType(plan.InstanceType)
	targetCPU, targetMemoryGB, directResize, err := resolveVirtualMachineResizeTarget(updateCtx, r.client, nodeRaw, &plan, currentInstanceType)
	if err != nil {
		resp.Diagnostics.AddError("Build SmartCMP resize request", err.Error())
		return
	}
	if directResize || (desiredInstanceType != "" && desiredInstanceType != stringValueFromType(state.InstanceType) && desiredInstanceType != currentInstanceType) {
		if state.ID.ValueString() == "" {
			resp.Diagnostics.AddError("Resize SmartCMP virtual machine", "resource_id is unavailable for resize")
			return
		}

		if currentPowerState != "stopped" {
			if err := executeTrackedResourceOperation(updateCtx, r.client, state.ID.ValueString(), "stop", map[string]any{
				"resourceId": state.ID.ValueString(),
			}); err != nil {
				resp.Diagnostics.AddError("Stop SmartCMP virtual machine", err.Error())
				return
			}
			currentPowerState = "stopped"
		}

		resizeParams := buildVirtualMachineResizeParameters(nodeRaw, &state, desiredInstanceType, targetCPU, targetMemoryGB, directResize)
		if err := executeTrackedResourceOperation(updateCtx, r.client, state.ID.ValueString(), "resize", resizeParams); err != nil {
			resp.Diagnostics.AddError("Resize SmartCMP virtual machine", err.Error())
			return
		}

		desiredPostResizePowerState := currentPowerState
		if hasDesiredPowerState {
			desiredPostResizePowerState = desiredPowerState
		} else if !plan.StartAfterResize.IsNull() && !plan.StartAfterResize.IsUnknown() && plan.StartAfterResize.ValueBool() {
			desiredPostResizePowerState = "started"
		}
		currentPowerState, err = reconcileVirtualMachinePowerState(updateCtx, r.client, state.ID.ValueString(), currentPowerState, desiredPostResizePowerState)
		if err != nil {
			resp.Diagnostics.AddError("Reconcile SmartCMP virtual machine power state", err.Error())
			return
		}
	} else if hasDesiredPowerState {
		currentPowerState, err = reconcileVirtualMachinePowerState(updateCtx, r.client, state.ID.ValueString(), currentPowerState, desiredPowerState)
		if err != nil {
			resp.Diagnostics.AddError("Reconcile SmartCMP virtual machine power state", err.Error())
			return
		}
	}

	nodeRaw, err = r.readOrResolveVirtualMachine(updateCtx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Read SmartCMP virtual machine", err.Error())
		return
	}
	applyVirtualMachineNodeRaw(&plan, nodeRaw)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VirtualMachineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured provider", "The provider client was not configured.")
		return
	}

	var data VirtualMachineResourceModel
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

	resourceID := data.ID.ValueString()
	if resourceID == "" {
		resourceID = data.ResourceID.ValueString()
	}
	if resourceID == "" {
		return
	}

	if err := executeTrackedResourceOperation(deleteCtx, r.client, resourceID, "tear_down_in_resource", map[string]any{
		"resourceId": resourceID,
	}); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Delete SmartCMP virtual machine", err.Error())
	}
}

func buildVirtualMachineRequestPayload(ctx context.Context, data *VirtualMachineResourceModel) (map[string]any, error) {
	payload := map[string]any{
		"catalogId":       data.CatalogID.ValueString(),
		"businessGroupId": data.BusinessGroupID.ValueString(),
		"name":            data.Name.ValueString(),
	}
	setString(payload, "description", data.Description)
	setNestedString(payload, data.Description, "genericRequest", "description")
	setString(payload, "userId", data.RequestUserID)
	if requestUserID := stringValueFromType(data.RequestUserID); requestUserID != "" {
		payload["requestUserId"] = requestUserID
	}
	setString(payload, "resourceBundleId", data.ResourcePoolID)

	spec := map[string]any{
		"node": "Compute",
		"type": "cloudchef.nodes.Compute",
	}
	setString(spec, "computeProfileId", data.ComputeProfileID)
	if desired := stringValueFromType(data.InstanceType); desired != "" {
		spec["flavorId"] = desired
		spec["flavor_id"] = desired
		spec["cloudFlavorId"] = desired
	}
	setString(spec, "logicTemplateId", data.LogicTemplateID)
	setString(spec, "physicalTemplateId", data.PhysicalTemplateID)
	setString(spec, "templateId", data.TemplateID)
	setString(spec, "credentialUser", data.CredentialUser)
	setString(spec, "credentialPassword", data.CredentialPassword)
	setString(spec, "networkId", data.NetworkID)
	setString(spec, "subnetId", data.SubnetID)

	if securityGroups, err := parseOptionalStringList(ctx, data.SecurityGroupIDs, "security_group_ids"); err != nil {
		return nil, err
	} else if len(securityGroups) > 0 {
		spec["securityGroupIds"] = securityGroups
	}

	systemDisk, err := decodeOptionalDisk(data.SystemDisk, "system_disk")
	if err != nil {
		return nil, err
	}
	if len(systemDisk) > 0 {
		spec["systemDisk"] = systemDisk
	}

	dataDisks, err := decodeOptionalDiskList(ctx, data.DataDisks, "data_disks")
	if err != nil {
		return nil, err
	}
	if len(dataDisks) > 0 {
		spec["dataDisks"] = dataDisks
	}

	payload["resourceSpecs"] = []map[string]any{spec}
	return payload, nil
}

func decodeOptionalDisk(value types.Object, fieldName string) (map[string]any, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var disk virtualMachineSystemDiskModel
	diags := value.As(context.Background(), &disk, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, fmt.Errorf("decode %s: %v", fieldName, diags)
	}
	return encodeDisk(context.Background(), virtualMachineDiskModel{
		Size:         disk.Size,
		IsSystemDisk: disk.IsSystemDisk,
		VolumeType:   disk.VolumeType,
		DiskPolicy:   disk.DiskPolicy,
		DiskTags:     disk.DiskTags,
	}, fieldName)
}

func decodeOptionalDiskList(ctx context.Context, value types.List, fieldName string) ([]map[string]any, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var disks []virtualMachineDiskModel
	diags := value.ElementsAs(ctx, &disks, false)
	if diags.HasError() {
		return nil, fmt.Errorf("decode %s: %v", fieldName, diags)
	}
	result := make([]map[string]any, 0, len(disks))
	for index, disk := range disks {
		encoded, err := encodeDisk(ctx, disk, fmt.Sprintf("%s[%d]", fieldName, index))
		if err != nil {
			return nil, err
		}
		if len(encoded) > 0 {
			result = append(result, encoded)
		}
	}
	return result, nil
}

func encodeDisk(ctx context.Context, disk virtualMachineDiskModel, fieldName string) (map[string]any, error) {
	result := map[string]any{}
	setString(result, "name", disk.Name)
	if !disk.Size.IsNull() && !disk.Size.IsUnknown() {
		result["size"] = disk.Size.ValueInt64()
	}
	if !disk.IsSystemDisk.IsNull() && !disk.IsSystemDisk.IsUnknown() {
		result["is_system_disk"] = disk.IsSystemDisk.ValueBool()
	}
	setString(result, "volume_type", disk.VolumeType)
	setString(result, "disk_policy", disk.DiskPolicy)
	if tags, err := parseOptionalStringList(ctx, disk.DiskTags, fieldName+".disk_tags"); err != nil {
		return nil, err
	} else if len(tags) > 0 {
		result["disk_tags"] = tags
	}
	return result, nil
}

func (r *VirtualMachineResource) readOrResolveVirtualMachine(ctx context.Context, data *VirtualMachineResourceModel) (map[string]any, error) {
	resourceID := data.ID.ValueString()
	if resourceID == "" {
		resourceID = data.ResourceID.ValueString()
	}
	if resourceID != "" {
		var raw map[string]any
		if err := r.client.GetJSON(ctx, "/nodes/"+url.PathEscape(resourceID), nil, &raw); err == nil {
			return raw, nil
		} else if !isNotFoundError(err) {
			return nil, err
		}
	}

	if data.DeploymentID.IsNull() || data.DeploymentID.IsUnknown() || data.DeploymentID.ValueString() == "" {
		return nil, fmt.Errorf("resource_id is unknown and deployment_id is unavailable")
	}

	nodeRaw, err := waitForVirtualMachineNode(ctx, r.client, data.DeploymentID.ValueString())
	if err != nil {
		return nil, err
	}
	applyVirtualMachineNodeRaw(data, nodeRaw)
	return nodeRaw, nil
}

func waitForVirtualMachineNode(ctx context.Context, client *client.Client, deploymentID string) (map[string]any, error) {
	var latest map[string]any
	for {
		nodes, err := listDeploymentNodes(ctx, client, deploymentID)
		if err != nil {
			return nil, err
		}
		if node, ok := selectVirtualMachineNode(nodes); ok {
			latest = node
			return latest, nil
		}

		select {
		case <-ctx.Done():
			if latest != nil {
				return latest, ctx.Err()
			}
			return nil, fmt.Errorf("timed out waiting for a VM node to appear in deployment %s", deploymentID)
		case <-time.After(resourcePollInterval):
		}
	}
}

func waitForVirtualMachineNodeByName(ctx context.Context, client *client.Client, name string, minCreatedDate int64) (map[string]any, error) {
	var latest map[string]any
	for {
		nodes, err := listVirtualMachineNodes(ctx, client)
		if err != nil {
			return nil, err
		}
		if node, ok := selectVirtualMachineNodeByName(nodes, name, minCreatedDate); ok {
			latest = node
			return latest, nil
		}

		select {
		case <-ctx.Done():
			if latest != nil {
				return latest, ctx.Err()
			}
			return nil, fmt.Errorf("timed out waiting for a VM node named %q to appear", name)
		case <-time.After(resourcePollInterval):
		}
	}
}

func listDeploymentNodes(ctx context.Context, client *client.Client, deploymentID string) ([]map[string]any, error) {
	params := url.Values{}
	params.Set("deploymentId", deploymentID)
	params.Set("size", "200")

	var raw any
	if err := client.GetJSON(ctx, "/nodes/all-status", params, &raw); err != nil {
		return nil, err
	}
	return extractItems(raw), nil
}

func listVirtualMachineNodes(ctx context.Context, client *client.Client) ([]map[string]any, error) {
	params := url.Values{}
	params.Set("size", "500")

	var raw any
	if err := client.GetJSON(ctx, "/nodes/all-status", params, &raw); err != nil {
		return nil, err
	}
	return extractItems(raw), nil
}

func selectVirtualMachineNode(items []map[string]any) (map[string]any, bool) {
	for _, item := range items {
		resourceType := strings.ToLower(findFirstString(item, "resourceType"))
		componentType := strings.ToLower(findFirstString(item, "componentType"))
		if strings.Contains(resourceType, "machine") || strings.Contains(resourceType, "instance") || strings.Contains(componentType, "machine") || strings.Contains(componentType, "instance") {
			return item, true
		}
	}
	if len(items) == 0 {
		return nil, false
	}
	return items[0], true
}

func selectVirtualMachineNodeByName(items []map[string]any, name string, minCreatedDate int64) (map[string]any, bool) {
	targetName := strings.TrimSpace(name)
	if targetName == "" {
		return nil, false
	}

	var best map[string]any
	var bestCreatedAt int64
	for _, item := range items {
		if _, ok := selectVirtualMachineNode([]map[string]any{item}); !ok {
			continue
		}

		names := []string{
			findFirstString(item, "name", "resourceName", "displayName", "externalName"),
			findFirstString(asMap(item["exts"]), "customProperty", "external_name"),
		}

		match := false
		for _, candidate := range names {
			if strings.TrimSpace(candidate) == targetName {
				match = true
				break
			}
		}
		if !match {
			continue
		}

		createdAt := int64(numberValue(item["createdDate"]))
		if minCreatedDate > 0 && createdAt > 0 && createdAt < minCreatedDate {
			continue
		}
		if best == nil || createdAt >= bestCreatedAt {
			best = item
			bestCreatedAt = createdAt
		}
	}

	if best == nil {
		return nil, false
	}
	return best, true
}

func applyVirtualMachineNodeRaw(data *VirtualMachineResourceModel, raw map[string]any) {
	resourceID := findFirstString(raw, "id")
	if resourceID != "" {
		data.ID = types.StringValue(resourceID)
		data.ResourceID = types.StringValue(resourceID)
	}
	setStringIfUnset(&data.Name, findFirstString(raw, "resourceName", "name", "displayName", "externalName"))
	data.DeploymentID = nullableString(findFirstString(raw, "deploymentId"))
	status := findFirstString(raw, "status")
	data.Status = nullableString(status)
	// Keep power_state aligned with the last observed remote state so Terraform can
	// detect and reconcile start/stop drift on the next plan.
	data.PowerState = nullableString(normalizeVirtualMachinePowerState(status))
	data.InstanceTypeActual = nullableString(findFirstString(raw, "instanceType", "flavorId"))

	if cpu := numberValue(raw["cpu"]); cpu > 0 {
		data.CPU = types.Int64Value(int64(cpu))
	} else if cpu := numberValue(raw["numCPUs"]); cpu > 0 {
		data.CPU = types.Int64Value(int64(cpu))
	}

	if memoryGB := virtualMachineMemoryGB(raw); memoryGB > 0 {
		data.MemoryGB = types.Float64Value(memoryGB)
	}
}

func virtualMachineMemoryGB(raw map[string]any) float64 {
	for _, key := range []string{"memoryInGB", "memoryGB", "memory"} {
		if value := numberValue(raw[key]); value > 0 {
			return value
		}
	}
	if exts := asMap(raw["exts"]); len(exts) > 0 {
		for _, key := range []string{"memory", "memoryInGB"} {
			if value := numberValue(exts[key]); value > 0 {
				return value
			}
		}
	}
	return 0
}

func resolveResizeTarget(ctx context.Context, client *client.Client, nodeRaw map[string]any, desiredInstanceType string, cpuOverride types.Int64, memoryOverride types.Float64) (int64, float64, error) {
	cpu := int64(0)
	if !cpuOverride.IsNull() && !cpuOverride.IsUnknown() {
		cpu = cpuOverride.ValueInt64()
	}
	memoryGB := float64(0)
	if !memoryOverride.IsNull() && !memoryOverride.IsUnknown() {
		memoryGB = memoryOverride.ValueFloat64()
	}
	if cpu > 0 && memoryGB > 0 {
		return cpu, memoryGB, nil
	}

	resolvedCPU, resolvedMemoryGB, err := lookupInstanceTypeShape(ctx, client, nodeRaw, desiredInstanceType)
	if err == nil && resolvedCPU > 0 && resolvedMemoryGB > 0 {
		return resolvedCPU, resolvedMemoryGB, nil
	}

	if cpu > 0 && memoryGB > 0 {
		return cpu, memoryGB, nil
	}
	return 0, 0, fmt.Errorf("unable to infer cpu and memory_gb for instance_type %q; set cpu and memory_gb explicitly", desiredInstanceType)
}

func resolveVirtualMachineResizeTarget(ctx context.Context, client *client.Client, nodeRaw map[string]any, plan *VirtualMachineResourceModel, currentInstanceType string) (int64, float64, bool, error) {
	desiredCPU, desiredMemoryGB := desiredVirtualMachineShape(nodeRaw, plan.CPU, plan.MemoryGB)
	currentCPU := currentVirtualMachineCPU(nodeRaw)
	currentMemoryGB := virtualMachineMemoryGB(nodeRaw)

	if isDirectCPUResizePlatform(nodeRaw) {
		cpuChanged := desiredCPU > 0 && currentCPU > 0 && desiredCPU != currentCPU
		memoryChanged := desiredMemoryGB > 0 && currentMemoryGB > 0 && !floatEqual(desiredMemoryGB, currentMemoryGB)
		if cpuChanged || memoryChanged {
			return desiredCPU, desiredMemoryGB, true, nil
		}
	}

	desiredInstanceType := stringValueFromType(plan.InstanceType)
	if desiredInstanceType != "" && desiredInstanceType != currentInstanceType {
		targetCPU, targetMemoryGB, err := resolveResizeTarget(ctx, client, nodeRaw, desiredInstanceType, plan.CPU, plan.MemoryGB)
		if err != nil {
			return 0, 0, false, err
		}
		return targetCPU, targetMemoryGB, false, nil
	}

	return 0, 0, false, nil
}

func buildVirtualMachineResizeParameters(nodeRaw map[string]any, state *VirtualMachineResourceModel, desiredInstanceType string, targetCPU int64, targetMemoryGB float64, directCPUResize bool) map[string]any {
	resourceName := findFirstString(nodeRaw, "resourceName", "name", "displayName", "externalName")
	params := map[string]any{
		"mode":           "upgrade",
		"cpu":            targetCPU,
		"cpus":           targetCPU,
		"numCPUs":        targetCPU,
		"memory":         targetMemoryGB,
		"memoryGB":       targetMemoryGB,
		"memoryMB":       int64(math.Round(targetMemoryGB * 1024.0)),
		"originNumCPUs":  currentVirtualMachineCPU(nodeRaw),
		"originMemoryMB": int64(math.Round(virtualMachineMemoryGB(nodeRaw) * 1024.0)),
		"resourceId":     state.ID.ValueString(),
		"deploymentId":   state.DeploymentID.ValueString(),
		"resourceName":   resourceName,
		"updateAllocate": true,
	}
	if !directCPUResize && desiredInstanceType != "" {
		params["flavorId"] = desiredInstanceType
		params["flavor_id"] = desiredInstanceType
		params["cloudFlavorId"] = desiredInstanceType
	}
	return params
}

func desiredVirtualMachineShape(nodeRaw map[string]any, cpuOverride types.Int64, memoryOverride types.Float64) (int64, float64) {
	cpu := currentVirtualMachineCPU(nodeRaw)
	if !cpuOverride.IsNull() && !cpuOverride.IsUnknown() && cpuOverride.ValueInt64() > 0 {
		cpu = cpuOverride.ValueInt64()
	}

	memoryGB := virtualMachineMemoryGB(nodeRaw)
	if !memoryOverride.IsNull() && !memoryOverride.IsUnknown() && memoryOverride.ValueFloat64() > 0 {
		memoryGB = memoryOverride.ValueFloat64()
	}
	return cpu, memoryGB
}

func currentVirtualMachineCPU(nodeRaw map[string]any) int64 {
	if cpu := int64(numberValue(rawValue(nodeRaw, "cpu", "numCPUs"))); cpu > 0 {
		return cpu
	}
	if cpu := int64(numberValue(findStringValue(nodeRaw, "exts", "cpu"))); cpu > 0 {
		return cpu
	}
	if cpu := int64(numberValue(findStringValue(asMap(nodeRaw["exts"]), "customProperty", "cpu"))); cpu > 0 {
		return cpu
	}
	return 0
}

func isDirectCPUResizePlatform(nodeRaw map[string]any) bool {
	values := []string{
		findFirstString(nodeRaw, "cloudEntryTypeId", "resourceType", "componentType"),
		findFirstString(asMap(nodeRaw["cloudEntryType"]), "id", "genericCloudSuffixName"),
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		switch {
		case strings.Contains(value, "vsphere"):
			return true
		case strings.Contains(value, "fusioncompute"):
			return true
		}
	}
	return false
}

func rawValue(raw map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			return value
		}
	}
	return nil
}

func floatEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.000001
}

func lookupInstanceTypeShape(ctx context.Context, client *client.Client, nodeRaw map[string]any, desiredInstanceType string) (int64, float64, error) {
	cloudEntryID := findFirstString(nodeRaw, "cloudEntryId")
	zoneID := findFirstString(nodeRaw, "zoneId")
	if cloudEntryID == "" {
		return 0, 0, fmt.Errorf("cloudEntryId is unavailable on the current VM")
	}

	payload := map[string]any{
		"cloudEntryId":      cloudEntryID,
		"cloudResourceType": "yacmp:cloudentry:type:aliyun::instance-types",
		"queryProperties": map[string]any{
			"regionId": findFirstString(nodeRaw, "regionId"),
			"zoneId":   zoneID,
			"flavorId": desiredInstanceType,
		},
	}
	if resourceBundleID := findFirstString(nodeRaw, "resourceBundleId"); resourceBundleID != "" {
		queryProperties := asMap(payload["queryProperties"])
		queryProperties["resourceBundleId"] = resourceBundleID
		queryProperties["queryResourceBundle"] = false
	}

	var raw any
	if err := client.PostJSON(ctx, "/cloudprovider?action=queryCloudResource", nil, payload, &raw); err != nil {
		return 0, 0, err
	}
	items := extractItems(raw)
	if len(items) == 0 {
		return 0, 0, fmt.Errorf("cloud provider did not return shape information for %s", desiredInstanceType)
	}
	item := items[0]
	if got := findFirstString(item, "id"); got != "" && got != desiredInstanceType {
		for _, candidate := range items {
			if findFirstString(candidate, "id") == desiredInstanceType {
				item = candidate
				break
			}
		}
	}
	cpu := int64(numberValue(findStringValue(item, "properties", "numCPUs")))
	if cpu == 0 {
		cpu = int64(numberValue(item["numCPUs"]))
	}
	memoryMB := numberValue(findStringValue(item, "properties", "memoryMB"))
	if memoryMB == 0 {
		memoryMB = numberValue(item["memoryMB"])
	}
	if cpu == 0 || memoryMB == 0 {
		return 0, 0, fmt.Errorf("cloud provider returned incomplete shape information for %s", desiredInstanceType)
	}
	return cpu, memoryMB / 1024.0, nil
}

func findStringValue(raw map[string]any, parent, key string) any {
	return asMap(raw[parent])[key]
}

func executeTrackedResourceOperation(ctx context.Context, client *client.Client, resourceID, operationID string, executeParameters map[string]any) error {
	payload := map[string]any{
		"operationId":                  operationID,
		"resourceIds":                  resourceID,
		"executeParameters":            executeParameters,
		"scheduledTaskMetadataRequest": map[string]any{},
	}

	var batchRaw map[string]any
	if err := client.PostJSON(ctx, "/nodes/resource-operations", nil, payload, &batchRaw); err != nil {
		return err
	}

	taskRaw, err := extractTaskFromBatchResponse(batchRaw, resourceID)
	if err != nil {
		return err
	}

	taskID := findFirstString(taskRaw, "id")
	if taskID == "" {
		return fmt.Errorf("resource operation %q did not return a task id", operationID)
	}

	latest, err := waitForTaskTerminal(ctx, client, taskID)
	if err != nil {
		return err
	}

	if state := findFirstString(latest, "state"); !strings.EqualFold(state, "FINISHED") {
		return fmt.Errorf("resource operation %q finished in state %s: %s", operationID, state, findFirstString(latest, "resultMsg", "message"))
	}
	return nil
}

func resolveDesiredPowerState(value types.String) (string, bool, error) {
	if value.IsNull() || value.IsUnknown() {
		return "", false, nil
	}

	powerState := normalizeVirtualMachinePowerState(value.ValueString())
	switch powerState {
	case "started", "stopped":
		return powerState, true, nil
	default:
		return "", false, fmt.Errorf(`power_state must be "started" or "stopped"`)
	}
}

func normalizeVirtualMachinePowerState(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "":
		return ""
	case strings.Contains(normalized, "stop"):
		return "stopped"
	case strings.Contains(normalized, "start"), strings.Contains(normalized, "run"), strings.Contains(normalized, "active"), strings.Contains(normalized, "poweron"):
		return "started"
	default:
		return normalized
	}
}

func reconcileVirtualMachinePowerState(ctx context.Context, client *client.Client, resourceID, currentPowerState, desiredPowerState string) (string, error) {
	if resourceID == "" || desiredPowerState == "" {
		return currentPowerState, nil
	}

	current := normalizeVirtualMachinePowerState(currentPowerState)
	desired := normalizeVirtualMachinePowerState(desiredPowerState)
	if current == desired {
		return current, nil
	}

	switch desired {
	case "started":
		if err := executeTrackedResourceOperation(ctx, client, resourceID, "start", map[string]any{
			"resourceId": resourceID,
		}); err != nil {
			return current, err
		}
	case "stopped":
		if err := executeTrackedResourceOperation(ctx, client, resourceID, "stop", map[string]any{
			"resourceId": resourceID,
		}); err != nil {
			return current, err
		}
	default:
		return current, fmt.Errorf("unsupported power_state %q", desiredPowerState)
	}

	return desired, nil
}

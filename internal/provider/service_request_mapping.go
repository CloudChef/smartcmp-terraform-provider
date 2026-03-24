package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type serviceRequestResourceSpecModel struct {
	Node                   types.String `tfsdk:"node"`
	Type                   types.String `tfsdk:"type"`
	ResourcePoolID         types.String `tfsdk:"resource_pool_id"`
	ResourcePoolTags       types.List   `tfsdk:"resource_pool_tags"`
	ResourcePoolParamsJSON types.String `tfsdk:"resource_pool_params_json"`
	SpecJSON               types.String `tfsdk:"spec_json"`
}

type serviceRequestResourceDefaults struct {
	ResourcePoolID     string
	ResourcePoolTags   []string
	ResourcePoolParams map[string]any
}

func buildServiceRequestPayload(ctx context.Context, data ServiceRequestResourceModel) (map[string]any, error) {
	payload, err := parseOptionalJSONObject(data.RequestBodyJSON, "request_body_json")
	if err != nil {
		return nil, err
	}
	payload = cloneMap(payload)

	defaults, err := buildServiceRequestResourceDefaults(ctx, data.ResourcePoolID, data.ResourcePoolTags, data.ResourcePoolParamsJSON, "resource_pool_params_json")
	if err != nil {
		return nil, err
	}

	setString(payload, "catalogId", data.CatalogID)
	setString(payload, "businessGroupId", data.BusinessGroupID)
	setString(payload, "name", data.Name)
	setString(payload, "description", data.Description)
	setNestedString(payload, data.Description, "genericRequest", "description")
	setString(payload, "projectId", data.ProjectID)
	if projectID := stringValueFromType(data.ProjectID); projectID != "" {
		if !hasMeaningfulValue(payload["groupId"]) {
			payload["groupId"] = projectID
		}
	}
	setString(payload, "userId", data.RequestUserID)
	if requestUserID := stringValueFromType(data.RequestUserID); requestUserID != "" {
		if !hasMeaningfulValue(payload["requestUserId"]) {
			payload["requestUserId"] = requestUserID
		}
	}
	setInt64(payload, "count", data.Count)
	setBool(payload, "addToInventory", data.AddToInventory)
	applyResourceDefaultsToMap(payload, defaults, false)

	typedSpecs, err := decodeTypedResourceSpecs(ctx, data.ResourceSpecs)
	if err != nil {
		return nil, err
	}

	if len(typedSpecs) > 0 {
		if _, exists := payload["resourceSpecs"]; exists {
			return nil, errors.New("resource_specs cannot be used together with request_body_json.resourceSpecs")
		}

		builtSpecs, err := buildTypedResourceSpecs(ctx, typedSpecs, defaults)
		if err != nil {
			return nil, err
		}
		payload["resourceSpecs"] = builtSpecs
		return payload, nil
	}

	if rawSpecs, ok := payload["resourceSpecs"]; ok {
		payload["resourceSpecs"] = fillResourceDefaultsOnRawSpecs(rawSpecs, defaults)
	}

	return payload, nil
}

func decodeTypedResourceSpecs(ctx context.Context, value types.List) ([]serviceRequestResourceSpecModel, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var specs []serviceRequestResourceSpecModel
	diags := value.ElementsAs(ctx, &specs, false)
	if diags.HasError() {
		return nil, fmt.Errorf("decode resource_specs: %v", diags)
	}

	return specs, nil
}

func buildTypedResourceSpecs(ctx context.Context, specs []serviceRequestResourceSpecModel, defaults serviceRequestResourceDefaults) ([]map[string]any, error) {
	if len(specs) > 1 {
		for index, spec := range specs {
			if stringValueFromType(spec.Node) == "" {
				return nil, fmt.Errorf("resource_specs[%d].node must be set when multiple resource_specs are configured", index)
			}
		}
	}

	result := make([]map[string]any, 0, len(specs))
	for index, spec := range specs {
		specPayload, err := parseOptionalJSONObject(spec.SpecJSON, fmt.Sprintf("resource_specs[%d].spec_json", index))
		if err != nil {
			return nil, err
		}
		specPayload = cloneMap(specPayload)

		specDefaults, err := buildServiceRequestResourceDefaults(ctx, spec.ResourcePoolID, spec.ResourcePoolTags, spec.ResourcePoolParamsJSON, fmt.Sprintf("resource_specs[%d].resource_pool_params_json", index))
		if err != nil {
			return nil, err
		}

		setString(specPayload, "node", spec.Node)
		setString(specPayload, "type", spec.Type)
		applyResourceDefaultsToMap(specPayload, specDefaults, false)
		applyResourceDefaultsToMap(specPayload, defaults, true)

		result = append(result, specPayload)
	}

	return result, nil
}

func buildServiceRequestResourceDefaults(ctx context.Context, resourcePoolID types.String, resourcePoolTags types.List, resourcePoolParamsJSON types.String, fieldName string) (serviceRequestResourceDefaults, error) {
	tags, err := parseOptionalStringList(ctx, resourcePoolTags, fieldName+".tags")
	if err != nil {
		return serviceRequestResourceDefaults{}, err
	}

	params, err := parseOptionalJSONObject(resourcePoolParamsJSON, fieldName)
	if err != nil {
		return serviceRequestResourceDefaults{}, err
	}

	return serviceRequestResourceDefaults{
		ResourcePoolID:     stringValueFromType(resourcePoolID),
		ResourcePoolTags:   append([]string(nil), tags...),
		ResourcePoolParams: params,
	}, nil
}

func fillResourceDefaultsOnRawSpecs(raw any, defaults serviceRequestResourceDefaults) any {
	switch typed := raw.(type) {
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			spec := cloneMap(asMap(item))
			applyResourceDefaultsToMap(spec, defaults, true)
			result = append(result, spec)
		}
		return result
	case []map[string]any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			spec := cloneMap(item)
			applyResourceDefaultsToMap(spec, defaults, true)
			result = append(result, spec)
		}
		return result
	default:
		return raw
	}
}

func applyResourceDefaultsToMap(target map[string]any, defaults serviceRequestResourceDefaults, onlyIfMissing bool) {
	if target == nil {
		return
	}

	if defaults.ResourcePoolID != "" && (!onlyIfMissing || !hasMeaningfulValue(target["resourceBundleId"])) {
		target["resourceBundleId"] = defaults.ResourcePoolID
	}
	if len(defaults.ResourcePoolTags) > 0 && (!onlyIfMissing || !hasMeaningfulValue(target["resourceBundleTags"])) {
		target["resourceBundleTags"] = append([]string(nil), defaults.ResourcePoolTags...)
	}
	if len(defaults.ResourcePoolParams) > 0 && (!onlyIfMissing || !hasMeaningfulValue(target["resourceBundleParams"])) {
		target["resourceBundleParams"] = cloneMap(defaults.ResourcePoolParams)
	}
}

func hasMeaningfulValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed != ""
	case []string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return stringValue(value) != ""
	}
}

func stringValueFromType(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

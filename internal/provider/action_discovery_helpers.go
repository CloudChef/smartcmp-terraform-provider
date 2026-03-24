package provider

import (
	"encoding/json"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type OperationActionItemModel struct {
	Operation            types.String `tfsdk:"operation"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	WebOperation         types.Bool   `tfsdk:"web_operation"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	DisabledMessage      types.String `tfsdk:"disabled_message"`
	MFA                  types.Bool   `tfsdk:"mfa"`
	SupportBatchAction   types.Bool   `tfsdk:"support_batch_action"`
	SupportScheduledTask types.Bool   `tfsdk:"support_scheduled_task"`
	SupportCharge        types.Bool   `tfsdk:"support_charge"`
	ActionCategoryName   types.String `tfsdk:"action_category_name"`
	ActionCategoryOrder  types.Int64  `tfsdk:"action_category_order"`
	SchemaJSON           types.String `tfsdk:"schema_json"`
	RawJSON              types.String `tfsdk:"raw_json"`
}

func operationActionSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"operation":              schema.StringAttribute{Computed: true},
		"name":                   schema.StringAttribute{Computed: true},
		"description":            schema.StringAttribute{Computed: true},
		"web_operation":          schema.BoolAttribute{Computed: true},
		"enabled":                schema.BoolAttribute{Computed: true},
		"disabled_message":       schema.StringAttribute{Computed: true},
		"mfa":                    schema.BoolAttribute{Computed: true},
		"support_batch_action":   schema.BoolAttribute{Computed: true},
		"support_scheduled_task": schema.BoolAttribute{Computed: true},
		"support_charge":         schema.BoolAttribute{Computed: true},
		"action_category_name":   schema.StringAttribute{Computed: true},
		"action_category_order":  schema.Int64Attribute{Computed: true},
		"schema_json":            schema.StringAttribute{Computed: true},
		"raw_json":               schema.StringAttribute{Computed: true},
	}
}

func operationActionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"operation":              types.StringType,
		"name":                   types.StringType,
		"description":            types.StringType,
		"web_operation":          types.BoolType,
		"enabled":                types.BoolType,
		"disabled_message":       types.StringType,
		"mfa":                    types.BoolType,
		"support_batch_action":   types.BoolType,
		"support_scheduled_task": types.BoolType,
		"support_charge":         types.BoolType,
		"action_category_name":   types.StringType,
		"action_category_order":  types.Int64Type,
		"schema_json":            types.StringType,
		"raw_json":               types.StringType,
	}
}

func mapOperationActionItem(item map[string]any) OperationActionItemModel {
	category := asMap(item["actionCategory"])

	return OperationActionItemModel{
		Operation:            nullableString(findFirstString(item, "id", "name")),
		Name:                 nullableString(findFirstString(item, "nameZh", "nameEn", "name", "id")),
		Description:          nullableString(findFirstString(item, "descriptionZh", "descriptionEn", "description", "disabledMsg")),
		WebOperation:         types.BoolValue(boolValue(item["webOperation"])),
		Enabled:              types.BoolValue(actionEnabled(item)),
		DisabledMessage:      nullableString(findFirstString(item, "disabledMsg")),
		MFA:                  types.BoolValue(boolValue(item["mfa"])),
		SupportBatchAction:   types.BoolValue(boolValue(item["supportBatchAction"])),
		SupportScheduledTask: types.BoolValue(boolValue(item["supportScheduledTask"])),
		SupportCharge:        types.BoolValue(boolValue(item["supportCharge"])),
		ActionCategoryName:   nullableString(findFirstString(category, "name", "id")),
		ActionCategoryOrder:  types.Int64Value(int64(numberValue(category["order"]))),
		SchemaJSON:           actionSchemaJSON(item),
		RawJSON:              jsonStringValue(item),
	}
}

func actionEnabled(item map[string]any) bool {
	if raw, ok := item["enabled"]; ok {
		return boolValue(raw)
	}
	return true
}

func actionSchemaJSON(item map[string]any) types.String {
	for _, key := range []string{"inputsForm", "parameters"} {
		raw, ok := item[key]
		if !ok || raw == nil {
			continue
		}

		switch typed := raw.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" || trimmed == "null" {
				continue
			}
			if json.Valid([]byte(trimmed)) {
				return types.StringValue(trimmed)
			}
			return types.StringValue(trimmed)
		default:
			return jsonStringValue(raw)
		}
	}

	return types.StringNull()
}

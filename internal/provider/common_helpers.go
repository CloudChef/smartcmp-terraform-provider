package provider

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

func configureDataSourceClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}

	providerData, err := mustProviderData(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected data source configure type", err.Error())
		return nil
	}

	return providerData.Client
}

func configureResourceClient(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}

	providerData, err := mustProviderData(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected resource configure type", err.Error())
		return nil
	}

	return providerData.Client
}

func jsonStringValue(value any) types.String {
	raw, err := json.Marshal(value)
	if err != nil {
		return types.StringValue("{}")
	}
	return types.StringValue(string(raw))
}

func listValueFromStructs[T any](ctx context.Context, objectType types.ObjectType, values []T) (types.List, diag.Diagnostics) {
	return types.ListValueFrom(ctx, objectType, values)
}

func asMap(value any) map[string]any {
	typed, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return typed
}

func extractItems(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, asMap(item))
		}
		return result
	case map[string]any:
		for _, key := range []string{"content", "data", "items", "result"} {
			if raw, ok := typed[key]; ok {
				return extractItems(raw)
			}
		}
		if _, ok := typed["id"]; ok {
			return []map[string]any{typed}
		}
	}

	return []map[string]any{}
}

func extractTotal(value any, fallback int) int64 {
	typed, ok := value.(map[string]any)
	if !ok {
		return int64(fallback)
	}

	for _, key := range []string{"totalElements", "total", "count"} {
		if raw, ok := typed[key]; ok {
			return int64(numberValue(raw))
		}
	}

	return int64(fallback)
}

func findFirstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := value[key]; ok {
			if str := stringValue(raw); str != "" {
				return str
			}
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true") || typed == "1"
	default:
		return false
	}
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		f, _ := typed.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(typed, 64)
		return f
	default:
		return 0
	}
}

func stringSliceValue(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func hashDataSourceID(prefix string, parts ...string) types.String {
	sum := sha1.Sum([]byte(prefix + "::" + strings.Join(parts, "::"))) //nolint:gosec
	return types.StringValue(prefix + ":" + hex.EncodeToString(sum[:8]))
}

func objectRawJSONAttribute() attr.Type {
	return types.StringType
}

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	smartcmpclient "github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

const (
	defaultResourceCreateTimeout = 30 * time.Minute
	defaultResourceReadTimeout   = 30 * time.Second
	defaultResourceDeleteTimeout = 5 * time.Minute
	resourcePollInterval         = 5 * time.Second
)

var (
	terminalRequestStates = map[string]struct{}{
		"FINISHED":          {},
		"FAILED":            {},
		"CANCELED":          {},
		"TIMEOUT_CLOSED":    {},
		"INITIALING_FAILED": {},
		"APPROVAL_REJECTED": {},
		"ARCHIVED":          {},
	}
	terminalTaskStates = map[string]struct{}{
		"FINISHED":          {},
		"FAILED":            {},
		"CANCELLED":         {},
		"APPROVAL_REJECTED": {},
	}
)

func parseOptionalJSONObject(value types.String, fieldName string) (map[string]any, error) {
	if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
		return map[string]any{}, nil
	}

	var raw any
	if err := json.Unmarshal([]byte(value.ValueString()), &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", fieldName, err)
	}

	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", fieldName)
	}

	return object, nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}

	raw, err := json.Marshal(input)
	if err != nil {
		result := make(map[string]any, len(input))
		for key, value := range input {
			result[key] = value
		}
		return result
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		result = make(map[string]any, len(input))
		for key, value := range input {
			result[key] = value
		}
	}

	return result
}

func setString(target map[string]any, key string, value types.String) {
	if target == nil || value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return
	}
	target[key] = value.ValueString()
}

func setInt64(target map[string]any, key string, value types.Int64) {
	if target == nil || value.IsNull() || value.IsUnknown() {
		return
	}
	target[key] = value.ValueInt64()
}

func setBool(target map[string]any, key string, value types.Bool) {
	if target == nil || value.IsNull() || value.IsUnknown() {
		return
	}
	target[key] = value.ValueBool()
}

func setNestedString(target map[string]any, value types.String, keys ...string) {
	if target == nil || value.IsNull() || value.IsUnknown() || value.ValueString() == "" || len(keys) == 0 {
		return
	}

	current := target
	for _, key := range keys[:len(keys)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}

	current[keys[len(keys)-1]] = value.ValueString()
}

func setNestedAny(target map[string]any, value any, keys ...string) {
	if target == nil || len(keys) == 0 {
		return
	}

	current := target
	for _, key := range keys[:len(keys)-1] {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}

	current[keys[len(keys)-1]] = value
}

func findStringPath(target map[string]any, keys ...string) string {
	current := any(target)
	for _, key := range keys {
		typed, ok := current.(map[string]any)
		if !ok {
			return ""
		}

		current, ok = typed[key]
		if !ok {
			return ""
		}
	}

	return stringValue(current)
}

func timeStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return timeStringValue(i)
		}
		return typed.String()
	case float64:
		return timeStringValue(int64(typed))
	case float32:
		return timeStringValue(int64(typed))
	case int:
		return time.UnixMilli(int64(typed)).UTC().Format(time.RFC3339)
	case int64:
		if typed <= 0 {
			return ""
		}
		if typed < 1_000_000_000_000 {
			return time.Unix(typed, 0).UTC().Format(time.RFC3339)
		}
		return time.UnixMilli(typed).UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

func nullableString(value string) types.String {
	if strings.TrimSpace(value) == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func setStringIfUnset(target *types.String, value string) {
	if target == nil || strings.TrimSpace(value) == "" {
		return
	}

	if target.IsNull() || target.IsUnknown() || strings.TrimSpace(target.ValueString()) == "" {
		*target = types.StringValue(value)
	}
}

func emptyStringList(ctx context.Context) (types.List, diag.Diagnostics) {
	return types.ListValue(types.StringType, []attr.Value{})
}

func stringListValue(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return emptyStringList(ctx)
	}

	return types.ListValueFrom(ctx, types.StringType, values)
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			stringified := stringValue(item)
			if stringified != "" {
				result = append(result, stringified)
			}
		}
		return result
	case string:
		return stringSliceValue(typed)
	default:
		return nil
	}
}

func parseOptionalStringList(ctx context.Context, value types.List, fieldName string) ([]string, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}

	var result []string
	diags := value.ElementsAs(ctx, &result, false)
	if diags.HasError() {
		return nil, fmt.Errorf("parse %s: %v", fieldName, diags)
	}

	filtered := make([]string, 0, len(result))
	for _, item := range result {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}

	return filtered, nil
}

func requestIsTerminal(state string) bool {
	_, ok := terminalRequestStates[strings.ToUpper(strings.TrimSpace(state))]
	return ok
}

func taskIsTerminal(state string) bool {
	_, ok := terminalTaskStates[strings.ToUpper(strings.TrimSpace(state))]
	return ok
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), " status 404") || strings.Contains(err.Error(), " returned status 404")
}

func fetchServiceRequest(ctx context.Context, client *smartcmpclient.Client, id string) (map[string]any, error) {
	var raw map[string]any
	if err := client.GetJSON(ctx, "/generic-request/"+url.PathEscape(id), nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func fetchTask(ctx context.Context, client *smartcmpclient.Client, id string) (map[string]any, error) {
	var raw map[string]any
	if err := client.GetJSON(ctx, "/tasks/"+url.PathEscape(id), nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func lookupDeploymentIDs(ctx context.Context, client *smartcmpclient.Client, requestID string) ([]string, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, nil
	}

	params := url.Values{}
	params.Set("query", "")
	params.Set("queryValue", "")
	params.Set("genericRequestId", requestID)
	params.Set("page", "1")
	params.Set("size", "200")

	var raw any
	if err := client.GetJSON(ctx, "/deployments", params, &raw); err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	result := []string{}
	for _, item := range extractItems(raw) {
		id := findFirstString(item, "id")
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result, nil
}

func populateServiceRequestDeploymentIDs(ctx context.Context, client *smartcmpclient.Client, data *ServiceRequestResourceModel) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	listValue, listDiags := emptyStringList(ctx)
	diags.Append(listDiags...)
	if !listDiags.HasError() {
		data.DeploymentIDs = listValue
	}

	if client == nil || data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		return "", diags
	}

	deploymentIDs, err := lookupDeploymentIDs(ctx, client, data.ID.ValueString())
	if err != nil {
		return err.Error(), diags
	}

	listValue, listDiags = stringListValue(ctx, deploymentIDs)
	diags.Append(listDiags...)
	if !listDiags.HasError() {
		data.DeploymentIDs = listValue
	}

	return "", diags
}

func waitForRequestTerminal(ctx context.Context, client *smartcmpclient.Client, id string) (map[string]any, error) {
	var latest map[string]any

	for {
		raw, err := fetchServiceRequest(ctx, client, id)
		if err != nil {
			return nil, err
		}

		latest = raw
		if requestIsTerminal(findFirstString(raw, "state")) {
			return latest, nil
		}

		select {
		case <-ctx.Done():
			return latest, ctx.Err()
		case <-time.After(resourcePollInterval):
		}
	}
}

func waitForTaskTerminal(ctx context.Context, client *smartcmpclient.Client, id string) (map[string]any, error) {
	var latest map[string]any

	for {
		raw, err := fetchTask(ctx, client, id)
		if err != nil {
			return nil, err
		}

		latest = raw
		if taskIsTerminal(findFirstString(raw, "state")) {
			return latest, nil
		}

		select {
		case <-ctx.Done():
			return latest, ctx.Err()
		case <-time.After(resourcePollInterval):
		}
	}
}

func applyServiceRequestRaw(data *ServiceRequestResourceModel, raw map[string]any) {
	data.ID = nullableString(findFirstString(raw, "id"))
	setStringIfUnset(&data.CatalogID, findFirstString(raw, "catalogId"))
	setStringIfUnset(&data.BusinessGroupID, findFirstString(raw, "businessGroupId"))
	setStringIfUnset(&data.Name, findFirstString(raw, "name", "requestName"))

	description := findFirstString(raw, "description")
	if description == "" {
		description = findStringPath(raw, "genericRequest", "description")
	}
	setStringIfUnset(&data.Description, description)

	groupID := findFirstString(raw, "projectId", "groupId")
	if groupID == "" {
		groupID = findStringPath(raw, "catalogServiceRequest", "projectId")
	}
	setStringIfUnset(&data.ProjectID, groupID)

	resourcePoolID := findFirstString(raw, "resourceBundleId")
	if resourcePoolID == "" {
		resourcePoolID = findStringPath(raw, "catalogServiceRequest", "resourceBundleId")
	}
	setStringIfUnset(&data.ResourcePoolID, resourcePoolID)

	data.State = nullableString(findFirstString(raw, "state"))
	data.ErrorMessage = nullableString(findFirstString(raw, "errMsg", "errorMessage", "message"))
	data.CompletedAt = nullableString(timeStringValue(raw["completedDate"]))
	setStringIfUnset(&data.RequestUserID, findFirstString(raw, "requestUserId", "userId"))
	data.InventoryID = nullableString(findFirstString(raw, "inventoryId"))
	data.ObjectID = nullableString(findFirstString(raw, "objectId"))
	data.ObjectType = nullableString(findFirstString(raw, "objectType"))
}

func applyTaskRaw(ctx context.Context, data *ResourceOperationResourceModel, raw map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics

	taskID := findFirstString(raw, "id")
	if taskID != "" {
		data.ID = types.StringValue(taskID)
		data.TaskID = types.StringValue(taskID)
	}

	if operation := findFirstString(raw, "operationName", "name"); operation != "" {
		setStringIfUnset(&data.Operation, operation)
	}
	if state := findFirstString(raw, "state"); state != "" {
		data.TaskState = types.StringValue(state)
	}

	data.TaskSubStage = nullableString(findFirstString(raw, "subStage"))
	data.ResultMessage = nullableString(findFirstString(raw, "resultMsg", "message"))
	data.GenericRequestID = nullableString(findFirstString(raw, "genericRequestId"))
	data.DeploymentID = nullableString(findFirstString(raw, "deploymentId"))

	resourceIDs := stringSliceFromAny(raw["resourceIds"])
	listValue, listDiags := stringListValue(ctx, resourceIDs)
	diags.Append(listDiags...)
	if !listDiags.HasError() {
		data.ResourceIDs = listValue
	}

	if targetID := findFirstString(raw, "deploymentId"); targetID != "" {
		setStringIfUnset(&data.TargetKind, "deployment")
		setStringIfUnset(&data.TargetID, targetID)
	} else if len(resourceIDs) > 0 {
		setStringIfUnset(&data.TargetKind, "resource")
		setStringIfUnset(&data.TargetID, resourceIDs[0])
	}

	return diags
}

func extractSingleRequest(raw any) (map[string]any, error) {
	items := extractItems(raw)
	if len(items) == 0 {
		return nil, fmt.Errorf("submit response did not include a generic request")
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("submit response returned %d generic requests; this provider currently supports exactly one request per resource", len(items))
	}
	return items[0], nil
}

func extractTaskFromBatchResponse(raw map[string]any, targetID string) (map[string]any, error) {
	if boolValue(raw["webOperation"]) {
		return nil, fmt.Errorf("the requested operation is interactive/web-based and is not supported by this Terraform resource")
	}

	results, ok := raw["results"].(map[string]any)
	if !ok || len(results) == 0 {
		return nil, fmt.Errorf("resource operation response did not include task results")
	}

	if task, ok := asMap(results[targetID])["id"]; ok && stringValue(task) != "" {
		return asMap(results[targetID]), nil
	}

	if candidate := asMap(results[targetID]); len(candidate) > 0 {
		return candidate, nil
	}

	if len(results) == 1 {
		for _, value := range results {
			candidate := asMap(value)
			if len(candidate) == 0 {
				continue
			}
			return candidate, nil
		}
	}

	return nil, fmt.Errorf("resource operation response did not contain a trackable task for target %q", targetID)
}

func isFullDeploymentOperationRequest(body map[string]any) bool {
	for _, key := range []string{
		"operationName",
		"scheduledTaskMetadataRequest",
		"operationParamJson",
		"params",
		"day2InventoryRequest",
		"resourceActionId",
	} {
		if _, ok := body[key]; ok {
			return true
		}
	}

	return false
}

func isFullResourceOperationRequest(body map[string]any) bool {
	for _, key := range []string{
		"operationId",
		"resourceIds",
		"scheduledTaskMetadataRequest",
		"executeParameters",
		"artifactParameters",
		"day2InventoryRequest",
		"sourceGenericId",
	} {
		if _, ok := body[key]; ok {
			return true
		}
	}

	return false
}

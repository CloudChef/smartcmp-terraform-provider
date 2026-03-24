package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

const (
	testUsername = "admin"
	testPassword = "example-password"
	testTenantID = "default"
)

type acceptanceConfig struct {
	BaseURL     string
	Username    string
	Password    string
	TenantID    string
	AuthMode    string
	CatalogName string
	Insecure    bool
}

func loadAcceptanceConfig(t *testing.T) acceptanceConfig {
	t.Helper()

	baseURL := os.Getenv("SMARTCMP_TEST_BASE_URL")
	username := os.Getenv("SMARTCMP_TEST_USERNAME")
	password := os.Getenv("SMARTCMP_TEST_PASSWORD")
	if baseURL == "" || username == "" || password == "" {
		t.Skip("set SMARTCMP_TEST_BASE_URL, SMARTCMP_TEST_USERNAME, and SMARTCMP_TEST_PASSWORD to run acceptance tests")
	}

	tenantID := os.Getenv("SMARTCMP_TEST_TENANT_ID")
	if tenantID == "" {
		tenantID = "default"
	}

	authMode := strings.TrimSpace(os.Getenv("SMARTCMP_TEST_AUTH_MODE"))
	if authMode == "" {
		authMode = client.AuthModeAuto
	}

	insecure := true
	if raw := strings.TrimSpace(os.Getenv("SMARTCMP_TEST_INSECURE")); raw != "" {
		insecure = raw == "1" || strings.EqualFold(raw, "true")
	}

	catalogName := os.Getenv("SMARTCMP_TEST_CATALOG_NAME")
	if catalogName == "" {
		catalogName = "Linux VM"
	}

	return acceptanceConfig{
		BaseURL:     baseURL,
		Username:    username,
		Password:    password,
		TenantID:    tenantID,
		AuthMode:    authMode,
		CatalogName: catalogName,
		Insecure:    insecure,
	}
}

func newTestClient(t *testing.T, baseURL, username, password, tenantID string, insecure bool) *client.Client {
	t.Helper()

	apiClient, err := client.New(client.Config{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		TenantID: tenantID,
		Insecure: insecure,
	})
	if err != nil {
		t.Fatalf("client.New returned error: %v", err)
	}

	return apiClient
}

func newLoggedInTestClient(t *testing.T, baseURL, username, password, tenantID string, insecure bool) *client.Client {
	t.Helper()

	apiClient := newTestClient(t, baseURL, username, password, tenantID, insecure)
	if err := apiClient.Login(context.Background()); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	return apiClient
}

func newConfiguredTestClient(t *testing.T, baseURL, username, password, tenantID, authMode string, insecure bool) *client.Client {
	t.Helper()

	p := New("test")()
	var schemaResp fwprovider.SchemaResponse
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, &schemaResp)

	values := map[string]tftypes.Value{
		"base_url":  tfStringValue(baseURL),
		"username":  tfStringValue(username),
		"password":  tfStringValue(password),
		"auth_mode": tfStringValue(authMode),
		"insecure":  tfBoolValue(insecure),
	}
	if tenantID != "" {
		values["tenant_id"] = tfStringValue(tenantID)
	}

	var resp fwprovider.ConfigureResponse
	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: newProviderConfig(t, schemaResp.Schema, values),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("provider configure failed: %v", resp.Diagnostics)
	}

	providerData, ok := resp.DataSourceData.(*ProviderData)
	if !ok || providerData == nil || providerData.Client == nil {
		t.Fatalf("expected configured provider client, got %T", resp.DataSourceData)
	}

	return providerData.Client
}

func newProviderConfig(t *testing.T, schema frameworkprovider.Schema, values map[string]tftypes.Value) tfsdk.Config {
	t.Helper()

	attributes := make(map[string]attributeTypeCarrier, len(schema.Attributes))
	for name, attribute := range schema.Attributes {
		attributes[name] = attribute
	}

	return tfsdk.Config{
		Raw:    newObjectValueFromAttributes(attributes, values),
		Schema: schema,
	}
}

func newProviderState(t *testing.T, schema frameworkprovider.Schema) tfsdk.State {
	t.Helper()

	return tfsdk.State{Schema: schema}
}

func newDataSourceReadRequest(t *testing.T, schema datasourceschema.Schema, values map[string]tftypes.Value) datasource.ReadRequest {
	t.Helper()

	attributes := make(map[string]attributeTypeCarrier, len(schema.Attributes))
	for name, attribute := range schema.Attributes {
		attributes[name] = attribute
	}

	return datasource.ReadRequest{
		Config: tfsdk.Config{
			Raw:    newObjectValueFromAttributes(attributes, values),
			Schema: schema,
		},
	}
}

func newDataSourceReadResponse(t *testing.T, schema datasourceschema.Schema) datasource.ReadResponse {
	t.Helper()

	return datasource.ReadResponse{
		State: tfsdk.State{Schema: schema},
	}
}

func newResourceCreateRequest(t *testing.T, schema resourceschema.Schema, values map[string]tftypes.Value) resource.CreateRequest {
	t.Helper()

	attributes := make(map[string]attributeTypeCarrier, len(schema.Attributes))
	for name, attribute := range schema.Attributes {
		attributes[name] = attribute
	}

	raw := newObjectValueFromAttributes(attributes, values)
	return resource.CreateRequest{
		Config: tfsdk.Config{
			Raw:    raw,
			Schema: schema,
		},
		Plan: tfsdk.Plan{
			Raw:    raw,
			Schema: schema,
		},
	}
}

func newResourceCreateResponse(t *testing.T, schema resourceschema.Schema) resource.CreateResponse {
	t.Helper()

	return resource.CreateResponse{
		State: tfsdk.State{Schema: schema},
	}
}

func newResourceReadRequest(t *testing.T, schema resourceschema.Schema, values map[string]tftypes.Value) resource.ReadRequest {
	t.Helper()

	attributes := make(map[string]attributeTypeCarrier, len(schema.Attributes))
	for name, attribute := range schema.Attributes {
		attributes[name] = attribute
	}

	return resource.ReadRequest{
		State: tfsdk.State{
			Raw:    newObjectValueFromAttributes(attributes, values),
			Schema: schema,
		},
	}
}

func newResourceReadResponse(t *testing.T, schema resourceschema.Schema) resource.ReadResponse {
	t.Helper()

	return resource.ReadResponse{
		State: tfsdk.State{Schema: schema},
	}
}

func newResourceDeleteRequest(t *testing.T, schema resourceschema.Schema, values map[string]tftypes.Value) resource.DeleteRequest {
	t.Helper()

	attributes := make(map[string]attributeTypeCarrier, len(schema.Attributes))
	for name, attribute := range schema.Attributes {
		attributes[name] = attribute
	}

	return resource.DeleteRequest{
		State: tfsdk.State{
			Raw:    newObjectValueFromAttributes(attributes, values),
			Schema: schema,
		},
	}
}

func newResourceDeleteResponse(t *testing.T, schema resourceschema.Schema) resource.DeleteResponse {
	t.Helper()

	return resource.DeleteResponse{
		State: tfsdk.State{Schema: schema},
	}
}

type attributeTypeCarrier interface {
	GetType() attr.Type
}

func newObjectValueFromAttributes(attributes map[string]attributeTypeCarrier, values map[string]tftypes.Value) tftypes.Value {
	attributeTypes := make(map[string]tftypes.Type, len(attributes))
	objectValues := make(map[string]tftypes.Value, len(attributes))
	for key, attribute := range attributes {
		tfType := attribute.GetType().TerraformType(context.Background())
		attributeTypes[key] = tfType
		if value, ok := values[key]; ok {
			objectValues[key] = value
			continue
		}
		objectValues[key] = tftypes.NewValue(tfType, nil)
	}
	return tftypes.NewValue(tftypes.Object{AttributeTypes: attributeTypes}, objectValues)
}

func tfStringValue(raw string) tftypes.Value {
	return tftypes.NewValue(tftypes.String, raw)
}

func tfBoolValue(raw bool) tftypes.Value {
	return tftypes.NewValue(tftypes.Bool, raw)
}

func tfInt64Value(raw int64) tftypes.Value {
	return tftypes.NewValue(tftypes.Number, raw)
}

func tfStringListValue(values ...string) tftypes.Value {
	items := make([]tftypes.Value, 0, len(values))
	for _, value := range values {
		items = append(items, tfStringValue(value))
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, items)
}

func tfObjectListValue(attributeTypes map[string]tftypes.Type, items []map[string]tftypes.Value) tftypes.Value {
	objectType := tftypes.Object{AttributeTypes: attributeTypes}
	values := make([]tftypes.Value, 0, len(items))
	for _, item := range items {
		objectValues := make(map[string]tftypes.Value, len(attributeTypes))
		for key, tfType := range attributeTypes {
			if value, ok := item[key]; ok {
				objectValues[key] = value
				continue
			}
			objectValues[key] = tftypes.NewValue(tfType, nil)
		}
		values = append(values, tftypes.NewValue(objectType, objectValues))
	}
	return tftypes.NewValue(tftypes.List{ElementType: objectType}, values)
}

func tfNullStringValue() tftypes.Value {
	return tftypes.NewValue(tftypes.String, nil)
}

func tfNullBoolValue() tftypes.Value {
	return tftypes.NewValue(tftypes.Bool, nil)
}

func encodeJSON(t *testing.T, value any) []byte {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	return raw
}

func newTLSServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(handler)
}

func captureQuery(values url.Values) url.Values {
	if values == nil {
		return nil
	}
	copyValues := make(url.Values, len(values))
	for key, slice := range values {
		copyValues[key] = append([]string(nil), slice...)
	}
	return copyValues
}

func maybeSkipUnreachableAcceptanceEndpoint(t *testing.T, cfg acceptanceConfig) {
	t.Helper()

	normalized, err := client.NormalizeBaseURL(cfg.BaseURL)
	if err != nil {
		t.Fatalf("normalize acceptance base url: %v", err)
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		t.Fatalf("parse acceptance base url: %v", err)
	}

	host := parsed.Host
	if !strings.Contains(host, ":") {
		switch parsed.Scheme {
		case "https":
			host += ":443"
		default:
			host += ":80"
		}
	}

	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		t.Skipf("acceptance endpoint not reachable over tcp at %s: %v", host, err)
	}
	_ = conn.Close()

	probeClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Insecure, //nolint:gosec
			},
		},
	}

	resp, err := probeClient.Get(normalized + "/custlogin/getCurrentUser")
	if err == nil {
		_ = resp.Body.Close()
		return
	}

	if isTransportUnreachableError(err) {
		t.Skipf("acceptance endpoint is configured but unreachable from this machine: %v", err)
	}
}

func isTransportUnreachableError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection refused",
		"no route to host",
		"network is unreachable",
		"timeout",
		"tls handshake timeout",
		"ssl_error_syscall",
		"handshake failure",
		"first record does not look like a tls handshake",
		"broken pipe",
		"connection reset by peer",
		"eof",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

func requireResourceSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("resource schema diagnostics: %v", resp.Diagnostics)
	}

	return resp.Schema
}

func dumpDiagnostics(diags fmt.Stringer) string {
	return diags.String()
}

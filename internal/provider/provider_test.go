package provider

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProviderTypeName(t *testing.T) {
	t.Parallel()

	p := New("test")().(*SmartCMPProvider)
	if p.version != "test" {
		t.Fatalf("expected version to be test, got %q", p.version)
	}
}

func TestProviderConfigureLogsIn(t *testing.T) {
	t.Parallel()

	var loginCount int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform-api/login" {
			t.Fatalf("expected login path, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST login, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			t.Fatalf("expected form login content-type, got %q", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		sum := md5.Sum([]byte(testPassword))
		if got := r.Form.Get("password"); got != hex.EncodeToString(sum[:]) {
			t.Fatalf("unexpected password hash: %q", got)
		}
		if got := r.Form.Get("encrypted"); got != "true" {
			t.Fatalf("unexpected encrypted flag: %q", got)
		}
		if got := r.Form.Get("tenant"); got != "default" {
			t.Fatalf("unexpected tenant: %q", got)
		}

		atomic.AddInt32(&loginCount, 1)
		http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New("test")()
	var schemaResp fwprovider.SchemaResponse
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, &schemaResp)

	var resp fwprovider.ConfigureResponse
	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: newProviderConfig(t, schemaResp.Schema, map[string]tftypes.Value{
			"base_url":        tfStringValue(server.URL),
			"username":        tfStringValue(testUsername),
			"password":        tfStringValue(testPassword),
			"tenant_id":       tfStringValue(testTenantID),
			"insecure":        tfBoolValue(true),
			"request_timeout": tfStringValue("2s"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if atomic.LoadInt32(&loginCount) != 1 {
		t.Fatalf("expected one login request, got %d", loginCount)
	}
	if resp.DataSourceData == nil || resp.ResourceData == nil {
		t.Fatalf("expected provider data to be wired to data sources and resources")
	}
	if _, ok := resp.DataSourceData.(*ProviderData); !ok {
		t.Fatalf("expected provider data type *ProviderData, got %T", resp.DataSourceData)
	}
}

func TestProviderConfigurePreservesCustomPlatformAPIPath(t *testing.T) {
	t.Parallel()

	var loginCount int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/platform-api/login" {
			t.Fatalf("expected login path, got %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if got := r.Form.Get("tenant"); got != "default" {
			t.Fatalf("unexpected tenant: %q", got)
		}

		atomic.AddInt32(&loginCount, 1)
		http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New("test")()
	var schemaResp fwprovider.SchemaResponse
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, &schemaResp)

	var resp fwprovider.ConfigureResponse
	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: newProviderConfig(t, schemaResp.Schema, map[string]tftypes.Value{
			"base_url":  tfStringValue(server.URL + "/custom/platform-api"),
			"username":  tfStringValue(testUsername),
			"password":  tfStringValue(testPassword),
			"tenant_id": tfStringValue(testTenantID),
			"insecure":  tfBoolValue(true),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if atomic.LoadInt32(&loginCount) != 1 {
		t.Fatalf("expected one login request, got %d", loginCount)
	}
}

func TestProviderConfigureUsesEnvironmentFallbacks(t *testing.T) {
	var loginCount int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform-api/login" {
			t.Fatalf("expected login path, got %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if got := r.Form.Get("tenant"); got != "default" {
			t.Fatalf("unexpected tenant: %q", got)
		}
		if got := r.Form.Get("encrypted"); got != "true" {
			t.Fatalf("unexpected encrypted flag: %q", got)
		}

		atomic.AddInt32(&loginCount, 1)
		http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("SMARTCMP_BASE_URL", server.URL)
	t.Setenv("SMARTCMP_USERNAME", testUsername)
	t.Setenv("SMARTCMP_PASSWORD", testPassword)
	t.Setenv("SMARTCMP_TENANT_ID", testTenantID)
	t.Setenv("SMARTCMP_AUTH_MODE", "private")
	t.Setenv("SMARTCMP_INSECURE", "true")

	p := New("test")()
	var schemaResp fwprovider.SchemaResponse
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, &schemaResp)

	var resp fwprovider.ConfigureResponse
	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: newProviderConfig(t, schemaResp.Schema, map[string]tftypes.Value{}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if atomic.LoadInt32(&loginCount) != 1 {
		t.Fatalf("expected one login request, got %d", loginCount)
	}
}

func TestTenantIDRequiredForSaaS(t *testing.T) {
	t.Parallel()

	if tenantIDRequired("https://console.smartcmp.cloud", "") {
		t.Fatalf("expected SaaS console host to allow empty tenant_id")
	}

	if !tenantIDRequired("https://cmp.internal.example", "") {
		t.Fatalf("expected on-prem host to require tenant_id")
	}

	if !tenantIDRequired("https://tenant.smartcmp.cloud:1443", "") {
		t.Fatalf("expected private smartcmp.cloud deployment to require tenant_id in auto mode")
	}
}

package client

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testPassword = "example-password"

func TestNormalizeBaseURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeBaseURL("cmp.internal.example")
	if err != nil {
		t.Fatalf("NormalizeBaseURL returned error: %v", err)
	}

	want := "https://cmp.internal.example/platform-api"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestInferAuthURL(t *testing.T) {
	t.Parallel()

	got := InferAuthURL("https://cmp.example.com/platform-api")
	want := "https://cmp.example.com/platform-api/login"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestInferAuthURLForSaaS(t *testing.T) {
	t.Parallel()

	got := InferAuthURL("https://console.smartcmp.cloud/platform-api")
	want := "https://account.smartcmp.cloud/bss-api/api/authentication"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestInferAuthURLTreatsPrivateSmartCMPSubdomainAsPrivate(t *testing.T) {
	t.Parallel()

	got := InferAuthURL("https://tenant.smartcmp.cloud:1443/platform-api")
	want := "https://tenant.smartcmp.cloud:1443/platform-api/login"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestInferAuthURLPreservesCustomPlatformAPIPrefix(t *testing.T) {
	t.Parallel()

	got := InferAuthURLForMode("https://cmp.example.com/custom/platform-api", AuthModePrivate)
	want := "https://cmp.example.com/custom/platform-api/login"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestNormalizePassword(t *testing.T) {
	t.Parallel()

	got := normalizePassword(testPassword)
	sum := md5.Sum([]byte(testPassword))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLoginSendsHashedPasswordAndTenant(t *testing.T) {
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
			t.Fatalf("expected form content-type, got %q", ct)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if got := r.Form.Get("username"); got != "admin" {
			t.Fatalf("expected username admin, got %q", got)
		}
		if got := r.Form.Get("tenant"); got != "default" {
			t.Fatalf("expected tenant default, got %q", got)
		}

		sum := md5.Sum([]byte(testPassword))
		wantPassword := hex.EncodeToString(sum[:])
		if got := r.Form.Get("password"); got != wantPassword {
			t.Fatalf("expected hashed password %q, got %q", wantPassword, got)
		}
		if got := r.Form.Get("encrypted"); got != "true" {
			t.Fatalf("expected encrypted=true, got %q", got)
		}

		atomic.AddInt32(&loginCount, 1)
		http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiClient, err := New(Config{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testPassword,
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := apiClient.Login(context.Background()); err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if atomic.LoadInt32(&loginCount) != 1 {
		t.Fatalf("expected exactly one login request, got %d", loginCount)
	}
}

func TestLoginOmitsTenantForSaaSAndChecksBusinessCode(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bss-api/api/authentication" {
			t.Fatalf("expected SaaS auth path, got %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		if got := r.Form.Get("tenant"); got != "" {
			t.Fatalf("expected tenant to be omitted, got %q", got)
		}
		sum := md5.Sum([]byte(testPassword))
		if got := r.Form.Get("password"); got != hex.EncodeToString(sum[:]) {
			t.Fatalf("expected hashed password %q, got %q", hex.EncodeToString(sum[:]), got)
		}
		if got := r.Form.Get("encrypted"); got != "" {
			t.Fatalf("expected encrypted to be omitted, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    "00000",
			"message": "ok",
			"data":    "https://console.smartcmp.cloud",
		})
	}))
	defer server.Close()

	apiClient, err := New(Config{
		BaseURL:  strings.Replace(server.URL, "https://", "https://console.smartcmp.cloud/", 1),
		Username: "admin",
		Password: testPassword,
		AuthMode: AuthModeSaaS,
		Insecure: true,
	})
	if err == nil {
		apiClient.authURL = server.URL + "/bss-api/api/authentication"
	}
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := apiClient.Login(context.Background()); err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
}

func TestResolveAuthMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		authMode string
		wantMode string
	}{
		{
			name:     "auto private host stays private",
			baseURL:  "https://tenant.smartcmp.cloud:1443/platform-api",
			authMode: "",
			wantMode: AuthModePrivate,
		},
		{
			name:     "auto public console becomes saas",
			baseURL:  "https://console.smartcmp.cloud/platform-api",
			authMode: "",
			wantMode: AuthModeSaaS,
		},
		{
			name:     "explicit private wins",
			baseURL:  "https://console.smartcmp.cloud/platform-api",
			authMode: AuthModePrivate,
			wantMode: AuthModePrivate,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := ResolveAuthMode(testCase.baseURL, testCase.authMode)
			if got != testCase.wantMode {
				t.Fatalf("expected %q, got %q", testCase.wantMode, got)
			}
		})
	}
}

func TestLoginRejectsBusinessFailureResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    "20008",
			"message": "Username or password is incorrect",
		})
	}))
	defer server.Close()

	apiClient, err := New(Config{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testPassword,
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	apiClient.authURL = server.URL

	err = apiClient.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Username or password is incorrect") {
		t.Fatalf("expected business login error, got %v", err)
	}
}

func TestGetJSONRetriesAfter401(t *testing.T) {
	t.Parallel()

	var loginCount int32
	var getCount int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/platform-api/login":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST login, got %s", r.Method)
			}
			atomic.AddInt32(&loginCount, 1)
			http.SetCookie(w, &http.Cookie{Name: "SESSION", Value: "ok", Path: "/"})
			w.WriteHeader(http.StatusOK)
		case "/platform-api/catalogs/published":
			atomic.AddInt32(&getCount, 1)
			if cookie, _ := r.Cookie("SESSION"); cookie == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content": []any{
					map[string]any{"id": "catalog-1", "nameZh": "Linux VM"},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient, err := New(Config{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testPassword,
		TenantID: "default",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	var got any
	if err := apiClient.GetJSON(context.Background(), "/catalogs/published", nil, &got); err != nil {
		t.Fatalf("GetJSON returned error: %v", err)
	}
	if atomic.LoadInt32(&loginCount) != 1 {
		t.Fatalf("expected one re-login after 401, got %d", loginCount)
	}
	if atomic.LoadInt32(&getCount) != 2 {
		t.Fatalf("expected two GET attempts, got %d", getCount)
	}
}

func TestProxyBypassForLocalHostname(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:8080")
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "smartcmp.local:443"}}

	got, err := proxyFromEnvironmentOrBypassPrivate(req)
	if err != nil {
		t.Fatalf("proxyFromEnvironmentOrBypassPrivate returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected proxy to be bypassed for local hostnames, got %v", got)
	}
}

func TestNewUsesDefaultTimeout(t *testing.T) {
	t.Parallel()

	client, err := New(Config{
		BaseURL:  "https://cmp.example.com",
		Username: "user",
		Password: "password",
		TenantID: "default",
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if client.http.Timeout != 30*time.Second {
		t.Fatalf("expected default timeout to be 30s, got %s", client.http.Timeout)
	}
}

func TestResolveURLPreservesPlatformAPIBasePath(t *testing.T) {
	t.Parallel()

	client, err := New(Config{
		BaseURL:  "https://cmp.example.com",
		Username: "user",
		Password: "password",
		TenantID: "default",
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	got, err := client.resolveURL("/catalogs/published", url.Values{"query": []string{""}})
	if err != nil {
		t.Fatalf("resolveURL returned error: %v", err)
	}

	want := "https://cmp.example.com/platform-api/catalogs/published?query="
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

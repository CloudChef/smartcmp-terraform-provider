package client

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout  = 30 * time.Second
	userAgent       = "terraform-provider-smartcmp/dev"
	AuthModeAuto    = "auto"
	AuthModePrivate = "private"
	AuthModeSaaS    = "saas"
)

var (
	ErrUnauthorized = errors.New("smartcmp: unauthorized")
)

type Config struct {
	BaseURL        string
	Username       string
	Password       string
	TenantID       string
	AuthMode       string
	Insecure       bool
	RequestTimeout time.Duration
}

type Client struct {
	baseURL   string
	authURL   string
	authMode  string
	username  string
	password  string
	tenantID  string
	http      *http.Client
	loginMu   sync.Mutex
	userAgent string
}

func New(cfg Config) (*Client, error) {
	baseURL, err := NormalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	httpClient := &http.Client{
		Timeout:   timeout,
		Jar:       jar,
		Transport: newTransport(cfg.Insecure),
	}

	authMode := ResolveAuthMode(baseURL, cfg.AuthMode)

	return &Client{
		baseURL:   baseURL,
		authURL:   InferAuthURLForMode(baseURL, authMode),
		authMode:  authMode,
		username:  cfg.Username,
		password:  cfg.Password,
		tenantID:  cfg.TenantID,
		http:      httpClient,
		userAgent: userAgent,
	}, nil
}

func NormalizeBaseURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("base_url is required")
	}

	candidate := strings.TrimSpace(raw)
	if !strings.HasPrefix(candidate, "http://") && !strings.HasPrefix(candidate, "https://") {
		candidate = "https://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("parse base_url: %w", err)
	}
	if parsed.Host == "" {
		return "", errors.New("base_url must include a host")
	}

	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	if !strings.HasSuffix(parsed.Path, "/platform-api") {
		parsed.Path += "/platform-api"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return strings.TrimSuffix(parsed.String(), "/"), nil
}

func InferAuthURL(baseURL string) string {
	return InferAuthURLForMode(baseURL, AuthModeAuto)
}

func InferAuthURLForMode(baseURL string, requestedMode string) string {
	normalizedBaseURL, err := NormalizeBaseURL(baseURL)
	if err != nil {
		normalizedBaseURL = baseURL
	}

	base, err := url.Parse(normalizedBaseURL)
	if err != nil {
		return baseURL + "/login"
	}

	if ResolveAuthMode(baseURL, requestedMode) == AuthModeSaaS {
		return inferSaaSAuthURL(strings.ToLower(base.Hostname()))
	}

	base.Path = strings.TrimSuffix(base.Path, "/") + "/login"
	base.RawQuery = ""
	base.Fragment = ""
	return base.String()
}

func ResolveAuthMode(baseURL string, requestedMode string) string {
	switch normalizeAuthMode(requestedMode) {
	case AuthModePrivate:
		return AuthModePrivate
	case AuthModeSaaS:
		return AuthModeSaaS
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return AuthModePrivate
	}

	// Only the public SmartCMP console/account hosts should auto-switch to SaaS auth.
	// Private deployments may still be served from smartcmp.cloud subdomains.
	if isCanonicalSaaSHost(strings.ToLower(base.Hostname())) {
		return AuthModeSaaS
	}

	return AuthModePrivate
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) Login(ctx context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	values := url.Values{}
	values.Set("username", c.username)
	values.Set("password", normalizePassword(c.password))
	if c.authMode == AuthModePrivate {
		values.Set("encrypted", "true")
	}
	if c.requiresTenant() {
		values.Set("tenant", c.tenantID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authURL, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("read login response: %w", err)
	}

	if err := validateLoginResponse(body); err != nil {
		return err
	}

	return nil
}

func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, out)
}

func (c *Client) PostJSON(ctx context.Context, path string, query url.Values, body any, out any) error {
	return c.doJSON(ctx, http.MethodPost, path, query, body, out)
}

func (c *Client) PutJSON(ctx context.Context, path string, query url.Values, body any, out any) error {
	return c.doJSON(ctx, http.MethodPut, path, query, body, out)
}

func (c *Client) DeleteJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, http.MethodDelete, path, query, nil, out)
}

func (c *Client) doJSON(ctx context.Context, method string, path string, query url.Values, body any, out any) error {
	return c.doJSONWithRetry(ctx, method, path, query, body, out, true)
}

func (c *Client) doJSONWithRetry(ctx context.Context, method string, path string, query url.Values, body any, out any, canRetry bool) error {
	reqURL, err := c.resolveURL(path, query)
	if err != nil {
		return err
	}

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		payload = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, payload)
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		if !canRetry {
			return ErrUnauthorized
		}
		if err := c.Login(ctx); err != nil {
			return fmt.Errorf("reauthenticate after 401: %w", err)
		}
		return c.doJSONWithRetry(ctx, method, path, query, body, out, false)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s returned status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) resolveURL(path string, query url.Values) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path is required")
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		u, err := url.Parse(path)
		if err != nil {
			return "", fmt.Errorf("parse request url: %w", err)
		}
		if len(query) > 0 {
			u.RawQuery = query.Encode()
		}
		return u.String(), nil
	}

	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}

	joinedPath := strings.TrimSuffix(base.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	base.Path = strings.ReplaceAll(joinedPath, "//", "/")
	if len(query) > 0 {
		base.RawQuery = query.Encode()
	} else {
		base.RawQuery = ""
	}
	base.Fragment = ""

	return base.String(), nil
}

func normalizePassword(password string) string {
	if isHexMD5(password) {
		return password
	}

	digest := md5.Sum([]byte(password))
	return hex.EncodeToString(digest[:])
}

func validateLoginResponse(body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	if code := stringValue(payload["code"]); code != "" {
		if code == "00000" {
			return nil
		}
		message := stringValue(payload["message"])
		if message == "" {
			message = "login was rejected by SmartCMP"
		}
		return fmt.Errorf("login failed: %s (%s)", message, code)
	}

	if raw, ok := payload["loginSuccess"]; ok {
		if boolValue(raw) {
			return nil
		}
		message := stringValue(payload["errorMessage"])
		if message == "" {
			message = "login was rejected by SmartCMP"
		}
		return fmt.Errorf("login failed: %s", message)
	}

	return nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%v", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	default:
		return false
	}
}

func isHexMD5(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func normalizeAuthMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", AuthModeAuto:
		return AuthModeAuto
	case AuthModePrivate:
		return AuthModePrivate
	case AuthModeSaaS:
		return AuthModeSaaS
	default:
		return ""
	}
}

func inferSaaSAuthURL(host string) string {
	if host == "console.cloudchef.io" || host == "account.cloudchef.io" {
		return "https://account.cloudchef.io/bss-api/api/authentication"
	}
	return "https://account.smartcmp.cloud/bss-api/api/authentication"
}

func isCanonicalSaaSHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))

	// SmartCMP-owned domains can host either the public SaaS console or a
	// customer-specific private deployment. Auto mode only treats the canonical
	// public console endpoints as SaaS so private subdomains stay on /login.
	switch host {
	case "console.smartcmp.cloud", "account.smartcmp.cloud", "console.cloudchef.io", "account.cloudchef.io":
		return true
	default:
		return false
	}
}

func (c *Client) requiresTenant() bool {
	return c.authMode != AuthModeSaaS
}

func newTransport(insecure bool) *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = proxyFromEnvironmentOrBypassPrivate
	base.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecure, //nolint:gosec
	}
	return base
}

func proxyFromEnvironmentOrBypassPrivate(req *http.Request) (*url.URL, error) {
	host := req.URL.Hostname()
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return nil, nil
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) || ip.IsLoopback() {
			return nil, nil
		}
	}

	return http.ProxyFromEnvironment(req)
}

func isPrivateIP(ip net.IP) bool {
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateCIDRs {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

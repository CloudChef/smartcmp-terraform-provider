package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/CloudChef/smartcmp-terraform-provider/internal/client"
)

var _ provider.Provider = &SmartCMPProvider{}

type SmartCMPProvider struct {
	version string
}

type SmartCMPProviderModel struct {
	BaseURL        types.String `tfsdk:"base_url"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	TenantID       types.String `tfsdk:"tenant_id"`
	AuthMode       types.String `tfsdk:"auth_mode"`
	Insecure       types.Bool   `tfsdk:"insecure"`
	RequestTimeout types.String `tfsdk:"request_timeout"`
}

type ProviderData struct {
	Client *client.Client
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &SmartCMPProvider{version: version}
	}
}

func (p *SmartCMPProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "smartcmp"
	resp.Version = p.version
}

func (p *SmartCMPProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		MarkdownDescription: "Terraform provider for CloudChef SmartCMP.",
		Attributes: map[string]providerschema.Attribute{
			"base_url": providerschema.StringAttribute{
				MarkdownDescription: "SmartCMP API base URL. The provider appends `/platform-api` if needed.",
				Optional:            true,
			},
			"username": providerschema.StringAttribute{
				MarkdownDescription: "SmartCMP login username.",
				Optional:            true,
			},
			"password": providerschema.StringAttribute{
				MarkdownDescription: "SmartCMP login password. Plaintext values are MD5-hashed before login to match SmartCMP client behavior.",
				Optional:            true,
				Sensitive:           true,
			},
			"tenant_id": providerschema.StringAttribute{
				MarkdownDescription: "SmartCMP tenant identifier. Required for private deployments and optional for the public SaaS console.",
				Optional:            true,
			},
			"auth_mode": providerschema.StringAttribute{
				MarkdownDescription: "Authentication mode: `auto`, `private`, or `saas`. Use `private` when a private deployment is served from a SmartCMP-owned domain such as `democmp.smartcmp.cloud`.",
				Optional:            true,
			},
			"insecure": providerschema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification for self-signed SmartCMP deployments.",
				Optional:            true,
			},
			"request_timeout": providerschema.StringAttribute{
				MarkdownDescription: "HTTP request timeout duration such as `30s` or `2m`.",
				Optional:            true,
			},
		},
	}
}

func (p *SmartCMPProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data SmartCMPProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := firstNonEmptyString(data.BaseURL, os.Getenv("SMARTCMP_BASE_URL"))
	username := firstNonEmptyString(data.Username, os.Getenv("SMARTCMP_USERNAME"))
	password := firstNonEmptyString(data.Password, os.Getenv("SMARTCMP_PASSWORD"))
	tenantID := firstNonEmptyString(data.TenantID, os.Getenv("SMARTCMP_TENANT_ID"))
	authMode := firstNonEmptyString(data.AuthMode, os.Getenv("SMARTCMP_AUTH_MODE"))
	authMode, err := normalizeProviderAuthMode(authMode)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("auth_mode"), "Invalid auth_mode", err.Error())
		return
	}

	tenantRequired := tenantIDRequired(baseURL, authMode)

	insecure := false
	if !data.Insecure.IsNull() && !data.Insecure.IsUnknown() {
		insecure = data.Insecure.ValueBool()
	} else if raw := os.Getenv("SMARTCMP_INSECURE"); raw == "true" || raw == "1" {
		insecure = true
	}

	timeout := 30 * time.Second
	if !data.RequestTimeout.IsNull() && !data.RequestTimeout.IsUnknown() {
		parsed, err := time.ParseDuration(data.RequestTimeout.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("request_timeout"), "Invalid request_timeout", err.Error())
			return
		}
		timeout = parsed
	}

	if baseURL == "" {
		resp.Diagnostics.AddAttributeError(path.Root("base_url"), "Missing base_url", "Set base_url or SMARTCMP_BASE_URL.")
	}
	if username == "" {
		resp.Diagnostics.AddAttributeError(path.Root("username"), "Missing username", "Set username or SMARTCMP_USERNAME.")
	}
	if password == "" {
		resp.Diagnostics.AddAttributeError(path.Root("password"), "Missing password", "Set password or SMARTCMP_PASSWORD.")
	}
	if tenantRequired && tenantID == "" {
		resp.Diagnostics.AddAttributeError(path.Root("tenant_id"), "Missing tenant_id", "Set tenant_id or SMARTCMP_TENANT_ID.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient, err := client.New(client.Config{
		BaseURL:        baseURL,
		Username:       username,
		Password:       password,
		TenantID:       tenantID,
		AuthMode:       authMode,
		Insecure:       insecure,
		RequestTimeout: timeout,
	})
	if err != nil {
		resp.Diagnostics.AddError("Configure SmartCMP client", err.Error())
		return
	}

	if err := apiClient.Login(ctx); err != nil {
		resp.Diagnostics.AddError("Login to SmartCMP", err.Error())
		return
	}

	providerData := &ProviderData{Client: apiClient}
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func tenantIDRequired(baseURL string, authMode string) bool {
	normalized, err := client.NormalizeBaseURL(baseURL)
	if err != nil {
		return true
	}

	return client.ResolveAuthMode(normalized, authMode) != client.AuthModeSaaS
}

func normalizeProviderAuthMode(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return client.AuthModeAuto, nil
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case client.AuthModeAuto:
		return client.AuthModeAuto, nil
	case client.AuthModePrivate:
		return client.AuthModePrivate, nil
	case client.AuthModeSaaS:
		return client.AuthModeSaaS, nil
	default:
		return "", fmt.Errorf("expected one of auto, private, or saas")
	}
}

func (p *SmartCMPProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewPublishedCatalogsDataSource,
		NewCatalogBusinessGroupsDataSource,
		NewCatalogComponentDataSource,
		NewResourcePoolsDataSource,
		NewApplicationsDataSource,
		NewOSTemplatesDataSource,
		NewCloudEntryTypesDataSource,
		NewImagesDataSource,
		NewProfilesDataSource,
		NewDeploymentActionsDataSource,
		NewResourceActionsDataSource,
		NewResourceActionsByIDsDataSource,
	}
}

func (p *SmartCMPProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewServiceRequestResource,
		NewResourceOperationResource,
	}
}

func firstNonEmptyString(value basetypes.StringValue, fallback string) string {
	if !value.IsNull() && !value.IsUnknown() && value.ValueString() != "" {
		return value.ValueString()
	}
	return fallback
}

func mustProviderData(data any) (*ProviderData, error) {
	providerData, ok := data.(*ProviderData)
	if !ok || providerData == nil || providerData.Client == nil {
		return nil, fmt.Errorf("provider not configured")
	}
	return providerData, nil
}

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAcceptanceProviderLoginAndProtectedRead(t *testing.T) {
	t.Parallel()

	cfg := loadAcceptanceConfig(t)
	maybeSkipUnreachableAcceptanceEndpoint(t, cfg)
	apiClient := newConfiguredTestClient(t, cfg.BaseURL, cfg.Username, cfg.Password, cfg.TenantID, cfg.AuthMode, cfg.Insecure)

	var raw any
	if err := apiClient.GetJSON(context.Background(), "/custlogin/getCurrentUser", nil, &raw); err != nil {
		t.Fatalf("expected protected read to succeed after login: %v", err)
	}
}

func TestAcceptanceLinuxVMCatalogAndProfiles(t *testing.T) {
	t.Parallel()

	cfg := loadAcceptanceConfig(t)
	maybeSkipUnreachableAcceptanceEndpoint(t, cfg)
	apiClient := newConfiguredTestClient(t, cfg.BaseURL, cfg.Username, cfg.Password, cfg.TenantID, cfg.AuthMode, cfg.Insecure)

	ctx := context.Background()
	var catalogsSchema datasource.SchemaResponse
	published := &PublishedCatalogsDataSource{client: apiClient}
	published.Schema(ctx, datasource.SchemaRequest{}, &catalogsSchema)

	publishedReq := newDataSourceReadRequest(t, catalogsSchema.Schema, map[string]tftypes.Value{
		"query": tfStringValue(cfg.CatalogName),
	})
	publishedResp := newDataSourceReadResponse(t, catalogsSchema.Schema)
	published.Read(ctx, publishedReq, &publishedResp)
	if publishedResp.Diagnostics.HasError() {
		t.Fatalf("published catalogs read failed: %v", publishedResp.Diagnostics)
	}

	var catalogState PublishedCatalogsDataSourceModel
	if diags := publishedResp.State.Get(ctx, &catalogState); diags.HasError() {
		t.Fatalf("catalog state decode failed: %v", diags)
	}
	var catalogs []PublishedCatalogItemModel
	if diags := catalogState.Items.ElementsAs(ctx, &catalogs, false); diags.HasError() {
		t.Fatalf("catalog items decode failed: %v", diags)
	}

	var sourceKey string
	for _, item := range catalogs {
		if strings.EqualFold(item.Name.ValueString(), cfg.CatalogName) {
			sourceKey = item.SourceKey.ValueString()
			break
		}
	}
	if sourceKey == "" {
		t.Fatalf("did not find catalog %q in published catalogs", cfg.CatalogName)
	}

	var profilesSchema datasource.SchemaResponse
	profiles := &ProfilesDataSource{client: apiClient}
	profiles.Schema(ctx, datasource.SchemaRequest{}, &profilesSchema)

	profilesReq := newDataSourceReadRequest(t, profilesSchema.Schema, map[string]tftypes.Value{
		"query":               tfNullStringValue(),
		"flavor_type":         tfStringValue("MACHINE"),
		"spec_type":           tfNullStringValue(),
		"catalog_id":          tfStringValue(sourceKey),
		"node_template_name":  tfStringValue("Compute"),
		"resource_bundle_id":  tfNullStringValue(),
		"cloud_entry_type_id": tfNullStringValue(),
		"provision_scope":     tfBoolValue(true),
	})
	profilesResp := newDataSourceReadResponse(t, profilesSchema.Schema)
	profiles.Read(ctx, profilesReq, &profilesResp)
	if profilesResp.Diagnostics.HasError() {
		t.Fatalf("profiles read failed: %v", profilesResp.Diagnostics)
	}

	var profilesState ProfilesDataSourceModel
	if diags := profilesResp.State.Get(ctx, &profilesState); diags.HasError() {
		t.Fatalf("profiles state decode failed: %v", diags)
	}
	var items []ProfileItemModel
	if diags := profilesState.Items.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatalf("profiles items decode failed: %v", diags)
	}
	if len(items) == 0 {
		t.Fatal("expected Linux VM profiles to return at least one item")
	}
	if items[0].Name.ValueString() == "" || items[0].CPU.ValueInt64() <= 0 || items[0].MemoryGB.ValueInt64() <= 0 {
		t.Fatalf("unexpected first profile payload: %+v", items[0])
	}
}

func TestAcceptanceDiscoveryChain(t *testing.T) {
	t.Parallel()

	cfg := loadAcceptanceConfig(t)
	maybeSkipUnreachableAcceptanceEndpoint(t, cfg)
	apiClient := newConfiguredTestClient(t, cfg.BaseURL, cfg.Username, cfg.Password, cfg.TenantID, cfg.AuthMode, cfg.Insecure)

	ctx := context.Background()

	var catalogsSchema datasource.SchemaResponse
	published := &PublishedCatalogsDataSource{client: apiClient}
	published.Schema(ctx, datasource.SchemaRequest{}, &catalogsSchema)

	publishedReq := newDataSourceReadRequest(t, catalogsSchema.Schema, map[string]tftypes.Value{
		"query": tfStringValue(cfg.CatalogName),
	})
	publishedResp := newDataSourceReadResponse(t, catalogsSchema.Schema)
	published.Read(ctx, publishedReq, &publishedResp)
	if publishedResp.Diagnostics.HasError() {
		t.Fatalf("published catalogs read failed: %v", publishedResp.Diagnostics)
	}

	var catalogState PublishedCatalogsDataSourceModel
	if diags := publishedResp.State.Get(ctx, &catalogState); diags.HasError() {
		t.Fatalf("catalog state decode failed: %v", diags)
	}
	var catalogs []PublishedCatalogItemModel
	if diags := catalogState.Items.ElementsAs(ctx, &catalogs, false); diags.HasError() {
		t.Fatalf("catalog items decode failed: %v", diags)
	}

	var catalogID string
	var sourceKey string
	for _, item := range catalogs {
		if strings.EqualFold(item.Name.ValueString(), cfg.CatalogName) {
			catalogID = item.ID.ValueString()
			sourceKey = item.SourceKey.ValueString()
			break
		}
	}
	if catalogID == "" || sourceKey == "" {
		t.Fatalf("did not find catalog %q in published catalogs", cfg.CatalogName)
	}

	var businessGroupsSchema datasource.SchemaResponse
	businessGroups := &CatalogBusinessGroupsDataSource{client: apiClient}
	businessGroups.Schema(ctx, datasource.SchemaRequest{}, &businessGroupsSchema)

	businessGroupsReq := newDataSourceReadRequest(t, businessGroupsSchema.Schema, map[string]tftypes.Value{
		"catalog_id": tfStringValue(catalogID),
	})
	businessGroupsResp := newDataSourceReadResponse(t, businessGroupsSchema.Schema)
	businessGroups.Read(ctx, businessGroupsReq, &businessGroupsResp)
	if businessGroupsResp.Diagnostics.HasError() {
		t.Fatalf("catalog business groups read failed: %v", businessGroupsResp.Diagnostics)
	}

	var businessGroupsState CatalogBusinessGroupsDataSourceModel
	if diags := businessGroupsResp.State.Get(ctx, &businessGroupsState); diags.HasError() {
		t.Fatalf("business groups state decode failed: %v", diags)
	}
	var groups []CatalogBusinessGroupItemModel
	if diags := businessGroupsState.Items.ElementsAs(ctx, &groups, false); diags.HasError() {
		t.Fatalf("business groups items decode failed: %v", diags)
	}
	if len(groups) == 0 {
		t.Fatalf("expected at least one business group for catalog %q", cfg.CatalogName)
	}

	var componentSchema datasource.SchemaResponse
	component := &CatalogComponentDataSource{client: apiClient}
	component.Schema(ctx, datasource.SchemaRequest{}, &componentSchema)

	componentReq := newDataSourceReadRequest(t, componentSchema.Schema, map[string]tftypes.Value{
		"source_key": tfStringValue(sourceKey),
	})
	componentResp := newDataSourceReadResponse(t, componentSchema.Schema)
	component.Read(ctx, componentReq, &componentResp)
	if componentResp.Diagnostics.HasError() {
		t.Fatalf("catalog component read failed: %v", componentResp.Diagnostics)
	}

	var componentState CatalogComponentDataSourceModel
	if diags := componentResp.State.Get(ctx, &componentState); diags.HasError() {
		t.Fatalf("component state decode failed: %v", diags)
	}
	if componentState.TypeName.IsNull() || componentState.TypeName.ValueString() == "" {
		t.Fatal("expected component type_name to be populated")
	}

	var cloudEntryTypesSchema datasource.SchemaResponse
	cloudEntryTypes := &CloudEntryTypesDataSource{client: apiClient}
	cloudEntryTypes.Schema(ctx, datasource.SchemaRequest{}, &cloudEntryTypesSchema)

	cloudEntryTypesReq := newDataSourceReadRequest(t, cloudEntryTypesSchema.Schema, map[string]tftypes.Value{})
	cloudEntryTypesResp := newDataSourceReadResponse(t, cloudEntryTypesSchema.Schema)
	cloudEntryTypes.Read(ctx, cloudEntryTypesReq, &cloudEntryTypesResp)
	if cloudEntryTypesResp.Diagnostics.HasError() {
		t.Fatalf("cloud entry types read failed: %v", cloudEntryTypesResp.Diagnostics)
	}

	var cloudEntryTypesState CloudEntryTypesDataSourceModel
	if diags := cloudEntryTypesResp.State.Get(ctx, &cloudEntryTypesState); diags.HasError() {
		t.Fatalf("cloud entry types state decode failed: %v", diags)
	}
	var entryTypes []CloudEntryTypeItemModel
	if diags := cloudEntryTypesState.Items.ElementsAs(ctx, &entryTypes, false); diags.HasError() {
		t.Fatalf("cloud entry types items decode failed: %v", diags)
	}
	if len(entryTypes) == 0 {
		t.Fatal("expected at least one cloud entry type")
	}

	var resourcePoolsSchema datasource.SchemaResponse
	resourcePools := &ResourcePoolsDataSource{client: apiClient}
	resourcePools.Schema(ctx, datasource.SchemaRequest{}, &resourcePoolsSchema)

	resourcePoolsReq := newDataSourceReadRequest(t, resourcePoolsSchema.Schema, map[string]tftypes.Value{
		"business_group_id": tfStringValue(groups[0].ID.ValueString()),
		"source_key":        tfStringValue(sourceKey),
		"node_type":         tfStringValue(componentState.TypeName.ValueString()),
	})
	resourcePoolsResp := newDataSourceReadResponse(t, resourcePoolsSchema.Schema)
	resourcePools.Read(ctx, resourcePoolsReq, &resourcePoolsResp)
	if resourcePoolsResp.Diagnostics.HasError() {
		t.Fatalf("resource pools read failed: %v", resourcePoolsResp.Diagnostics)
	}

	var applicationsSchema datasource.SchemaResponse
	applications := &ApplicationsDataSource{client: apiClient}
	applications.Schema(ctx, datasource.SchemaRequest{}, &applicationsSchema)

	applicationsReq := newDataSourceReadRequest(t, applicationsSchema.Schema, map[string]tftypes.Value{
		"business_group_id": tfStringValue(groups[0].ID.ValueString()),
		"query":             tfNullStringValue(),
	})
	applicationsResp := newDataSourceReadResponse(t, applicationsSchema.Schema)
	applications.Read(ctx, applicationsReq, &applicationsResp)
	if applicationsResp.Diagnostics.HasError() {
		t.Fatalf("applications read failed: %v", applicationsResp.Diagnostics)
	}
}

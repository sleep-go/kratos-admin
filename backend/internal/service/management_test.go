package service

import (
	"context"
	"testing"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
)

type fakeManagementRepository struct {
	scope          ResourceScope
	filters        map[string]string
	effectiveRows  []map[string]any
	providerTested uint64
	roleID         uint64
	roleScope      uint32
	tenantID       uint64
	featureIDs     []uint64
}

func (r *fakeManagementRepository) UpdateTenantFeatures(_ context.Context, _ ResourceScope, tenantID uint64, resourceIDs []uint64) error {
	r.tenantID, r.featureIDs = tenantID, resourceIDs
	return nil
}

func (r *fakeManagementRepository) UpdateRoleAuthorization(_ context.Context, _ ResourceScope, roleID uint64, dataScope uint32, _ []RoleGrant, _ []uint64) error {
	r.roleID, r.roleScope = roleID, dataScope
	return nil
}

type fakePermissionChecker struct{ allowed bool }

func (c fakePermissionChecker) Allowed(context.Context, ResourceScope, string, string) (bool, error) {
	return c.allowed, nil
}

func (r *fakeManagementRepository) List(_ context.Context, scope ResourceScope, _ string, _ PageQuery) ([]map[string]any, uint64, error) {
	r.scope = scope
	return []map[string]any{{"id": uint64(1), "name": "示例部门"}}, 1, nil
}
func (r *fakeManagementRepository) Create(context.Context, ResourceScope, string, map[string]any) (uint64, error) {
	return 1, nil
}
func (r *fakeManagementRepository) Update(context.Context, ResourceScope, string, uint64, map[string]any) error {
	return nil
}
func (r *fakeManagementRepository) Delete(context.Context, ResourceScope, string, uint64) error {
	return nil
}

func (r *fakeManagementRepository) EffectiveSettings(_ context.Context, scope ResourceScope, category string) ([]map[string]any, error) {
	r.scope = scope
	r.filters = map[string]string{"category": category}
	return r.effectiveRows, nil
}

func (r *fakeManagementRepository) TestProviderConnection(_ context.Context, scope ResourceScope, id uint64) error {
	r.scope = scope
	r.providerTested = id
	return nil
}

func TestManagementServiceAlwaysUsesAuthenticatedTenant(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9})

	reply, err := service.ListResources(ctx, &v1.ListResourcesRequest{
		Resource: "departments", Page: 1, PageSize: 20,
		Filters: map[string]string{"tenant_id": "999"},
	})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if repository.scope.TenantID != 8 || repository.scope.MemberID != 9 {
		t.Fatalf("scope = %+v", repository.scope)
	}
	if reply.Total != 1 || len(reply.Items) != 1 {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestManagementServiceRejectsPlatformResourceInTenantContext(t *testing.T) {
	service := NewManagementService(&fakeManagementRepository{})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9})

	if _, err := service.ListResources(ctx, &v1.ListResourcesRequest{Resource: "tenants"}); err == nil {
		t.Fatal("tenant context must not access platform tenant governance")
	}
}

func TestPlatformAdminCanGovernFromTenantContext(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository, fakePermissionChecker{allowed: false})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{
		UserID: 5, TenantID: 8, MemberID: 9, PlatformAdmin: true,
	})

	if _, err := service.ListResources(ctx, &v1.ListResourcesRequest{Resource: "login-logs"}); err != nil {
		t.Fatalf("platform admin ListResources() error = %v", err)
	}
	if !repository.scope.PlatformAdmin {
		t.Fatalf("scope = %+v", repository.scope)
	}
}

func TestManagementServiceRejectsMissingCasbinPermission(t *testing.T) {
	service := NewManagementService(&fakeManagementRepository{}, fakePermissionChecker{allowed: false})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9})

	if _, err := service.ListResources(ctx, &v1.ListResourcesRequest{Resource: "departments"}); err == nil {
		t.Fatal("missing list permission must be rejected")
	}
}

func TestGetEffectiveSettingsUsesAuthenticatedTenant(t *testing.T) {
	repository := &fakeManagementRepository{effectiveRows: []map[string]any{{"key": "site_name", "source": "tenant"}}}
	service := NewManagementService(repository)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9})

	reply, err := service.GetEffectiveSettings(ctx, &v1.GetEffectiveSettingsRequest{Category: "platform"})
	if err != nil {
		t.Fatal(err)
	}
	if repository.scope.TenantID != 8 || repository.filters["category"] != "platform" || len(reply.Items) != 1 {
		t.Fatalf("scope = %+v, filters = %+v, reply = %+v", repository.scope, repository.filters, reply)
	}
}

func TestProviderConnectionTestRequiresUpdatePermission(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository, fakePermissionChecker{allowed: false})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9})
	if _, err := service.TestProviderConnection(ctx, &v1.TestProviderConnectionRequest{Id: 7}); err == nil {
		t.Fatal("connection test must require provider update permission")
	}
	if repository.providerTested != 0 {
		t.Fatalf("providerTested = %d", repository.providerTested)
	}
}

func TestUpdateRoleAuthorizationIsSingleAuthorizedOperation(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository, fakePermissionChecker{allowed: true})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9})

	_, err := service.UpdateRoleAuthorization(ctx, &v1.UpdateRoleAuthorizationRequest{
		RoleId: 12, DataScope: 5,
		Grants:        []*v1.RoleResourceGrant{{ResourceCode: "files", Actions: []string{"list", "download"}}},
		DepartmentIds: []uint64{3, 4},
	})
	if err != nil || repository.roleID != 12 || repository.roleScope != 5 {
		t.Fatalf("UpdateRoleAuthorization() role=%d scope=%d err=%v", repository.roleID, repository.roleScope, err)
	}
}

func TestUpdateTenantFeaturesRequiresPlatformAdministrator(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, PlatformAdmin: true})

	_, err := service.UpdateTenantFeatures(ctx, &v1.UpdateTenantFeaturesRequest{TenantId: 10, ResourceIds: []uint64{2, 3}})
	if err != nil || repository.tenantID != 10 || len(repository.featureIDs) != 2 {
		t.Fatalf("UpdateTenantFeatures() tenant=%d resources=%v err=%v", repository.tenantID, repository.featureIDs, err)
	}
}

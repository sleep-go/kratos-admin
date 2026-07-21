package service

import (
	"context"
	"testing"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
)

type fakeManagementRepository struct {
	scope          managementbiz.Scope
	resource       string
	filters        map[string]string
	effectiveRows  []map[string]any
	providerTested uint64
	roleID         uint64
	roleScope      uint32
	tenantID       uint64
	featureIDs     []uint64
}

func (r *fakeManagementRepository) UpdateTenantFeatures(_ context.Context, _ managementbiz.Scope, tenantID uint64, resourceIDs []uint64) error {
	r.tenantID, r.featureIDs = tenantID, resourceIDs
	return nil
}

func (r *fakeManagementRepository) UpdateRoleAuthorization(_ context.Context, _ managementbiz.Scope, roleID uint64, dataScope uint32, _ []managementbiz.RoleGrant, _ []uint64) error {
	r.roleID, r.roleScope = roleID, dataScope
	return nil
}

type fakePermissionChecker struct{ allowed bool }

func (c fakePermissionChecker) Allowed(context.Context, managementbiz.Scope, string, string) (bool, error) {
	return c.allowed, nil
}

func (r *fakeManagementRepository) List(_ context.Context, scope managementbiz.Scope, _ string, _ managementbiz.PageQuery) ([]map[string]any, uint64, error) {
	r.scope = scope
	return []map[string]any{{"id": uint64(1), "name": "示例部门"}}, 1, nil
}
func (r *fakeManagementRepository) Create(_ context.Context, scope managementbiz.Scope, resource string, _ map[string]any) (uint64, error) {
	r.scope, r.resource = scope, resource
	return 1, nil
}
func (r *fakeManagementRepository) Update(context.Context, managementbiz.Scope, string, uint64, map[string]any) error {
	return nil
}
func (r *fakeManagementRepository) Delete(context.Context, managementbiz.Scope, string, uint64) error {
	return nil
}

func (r *fakeManagementRepository) EffectiveSettings(_ context.Context, scope managementbiz.Scope, category string) ([]map[string]any, error) {
	r.scope = scope
	r.filters = map[string]string{"category": category}
	return r.effectiveRows, nil
}

func (r *fakeManagementRepository) TestProviderConnection(_ context.Context, scope managementbiz.Scope, id uint64) error {
	r.scope = scope
	r.providerTested = id
	return nil
}

func TestManagementServiceAlwaysUsesAuthenticatedTenant(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant})

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
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant})

	if _, err := service.ListResources(ctx, &v1.ListResourcesRequest{Resource: "tenants"}); err == nil {
		t.Fatal("tenant context must not access platform tenant governance")
	}
}

func TestPlatformAdministratorInTenantContextDoesNotBypassPermission(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository, fakePermissionChecker{allowed: false})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{
		UserID: 5, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant, ImpersonatorID: 1,
	})

	if _, err := service.ListResources(ctx, &v1.ListResourcesRequest{Resource: "login-logs"}); err == nil {
		t.Fatal("平台管理员在租户上下文必须接受租户权限检查")
	}
	if repository.scope.PlatformAdmin {
		t.Fatalf("scope = %+v", repository.scope)
	}
}

func TestManagementServiceRejectsMissingCasbinPermission(t *testing.T) {
	service := NewManagementService(&fakeManagementRepository{}, fakePermissionChecker{allowed: false})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant})

	if _, err := service.ListResources(ctx, &v1.ListResourcesRequest{Resource: "departments"}); err == nil {
		t.Fatal("missing list permission must be rejected")
	}
}

func TestGetEffectiveSettingsUsesAuthenticatedTenant(t *testing.T) {
	repository := &fakeManagementRepository{effectiveRows: []map[string]any{{"key": "site_name", "source": "tenant"}}}
	service := NewManagementService(repository)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant})

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
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant})
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
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant})

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
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 0, Realm: bizauth.RealmPlatform})

	_, err := service.UpdateTenantFeatures(ctx, &v1.UpdateTenantFeaturesRequest{TenantId: 10, ResourceIds: []uint64{2, 3}})
	if err != nil || repository.tenantID != 10 || len(repository.featureIDs) != 2 {
		t.Fatalf("UpdateTenantFeatures() tenant=%d resources=%v err=%v", repository.tenantID, repository.featureIDs, err)
	}
}

func TestPlatformTenantSetupUsesExplicitTargetTenant(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 0, Realm: bizauth.RealmPlatform})

	_, err := service.CreateResource(ctx, &v1.CreateResourceRequest{
		Resource: "roles", TargetTenantId: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.scope.TenantID != 8 || !repository.scope.PlatformAdmin || repository.resource != "roles" {
		t.Fatalf("scope = %+v, resource = %q", repository.scope, repository.resource)
	}
}

func TestTenantContextCannotUsePlatformTenantSetupTarget(t *testing.T) {
	service := NewManagementService(&fakeManagementRepository{})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{
		UserID: 5, TenantID: 7, MemberID: 9, Realm: bizauth.RealmTenant, ImpersonatorID: 1,
	})

	if _, err := service.CreateResource(ctx, &v1.CreateResourceRequest{Resource: "roles", TargetTenantId: 8}); err == nil {
		t.Fatal("租户上下文不能指定平台初始化目标租户")
	}
}

func TestPlatformTenantSetupRejectsPlatformResource(t *testing.T) {
	service := NewManagementService(&fakeManagementRepository{})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 0, Realm: bizauth.RealmPlatform})

	if _, err := service.CreateResource(ctx, &v1.CreateResourceRequest{Resource: "users", TargetTenantId: 8}); err == nil {
		t.Fatal("平台专属资源不能通过目标租户初始化入口维护")
	}
}

func TestPlatformTenantSetupCanListTargetTenantFeatures(t *testing.T) {
	repository := &fakeManagementRepository{}
	service := NewManagementService(repository)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 5, TenantID: 0, Realm: bizauth.RealmPlatform})

	if _, err := service.ListResources(ctx, &v1.ListResourcesRequest{
		Resource: "tenant-resources", TargetTenantId: 8,
	}); err != nil {
		t.Fatal(err)
	}
	if repository.scope.TenantID != 8 || !repository.scope.PlatformAdmin {
		t.Fatalf("scope = %+v, want target tenant 8", repository.scope)
	}
}

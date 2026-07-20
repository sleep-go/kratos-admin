package service

import (
	"context"
	"testing"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
)

type fakeManagementRepository struct {
	scope   ResourceScope
	filters map[string]string
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

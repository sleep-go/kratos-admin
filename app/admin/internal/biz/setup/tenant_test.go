package setup

import (
	"context"
	"testing"
)

func TestTenantProvisionerRejectsZeroTenantID(t *testing.T) {
	provisioner := NewTenantProvisioner(&fakeTenantRepository{})
	if err := provisioner.EnsureDefaults(context.Background(), 0); err == nil {
		t.Fatal("EnsureDefaults() with tenantID=0 should error")
	}
}

func TestTenantProvisionerCreatesBuiltinRoleAndPolicy(t *testing.T) {
	repo := &fakeTenantRepository{}
	provisioner := NewTenantProvisioner(repo)

	if err := provisioner.EnsureDefaults(context.Background(), 42); err != nil {
		t.Fatalf("EnsureDefaults() error = %v", err)
	}
	if repo.createdRole == nil {
		t.Fatal("expected builtin role to be created")
	}
	if repo.createdRole.Code != "tenant-admin" || !repo.createdRole.IsBuiltin {
		t.Fatalf("unexpected role: %+v", repo.createdRole)
	}
	if repo.createdRole.TenantID != 42 || repo.createdRole.DataScope != 1 || repo.createdRole.Status != 1 {
		t.Fatalf("unexpected role fields: %+v", repo.createdRole)
	}
	if repo.createdCasbinRule == nil {
		t.Fatal("expected casbin policy to be created")
	}
	if repo.createdCasbinRule.Ptype != "p" || repo.createdCasbinRule.V2 != "*" || repo.createdCasbinRule.V3 != "*" {
		t.Fatalf("unexpected casbin rule: %+v", repo.createdCasbinRule)
	}
	if repo.createdCasbinRule.V0 != "42" || repo.createdCasbinRule.V1 != "999" {
		t.Fatalf("unexpected casbin rule v0/v1: %+v", repo.createdCasbinRule)
	}
}

func TestTenantProvisionerIsIdempotentWhenAllExists(t *testing.T) {
	repo := &fakeTenantRepository{existingRoleID: 100, existingPolicy: true}
	provisioner := NewTenantProvisioner(repo)

	if err := provisioner.EnsureDefaults(context.Background(), 42); err != nil {
		t.Fatalf("EnsureDefaults() error = %v", err)
	}
	if repo.createdRole != nil {
		t.Fatal("existing role must not be recreated")
	}
	if repo.createdCasbinRule != nil {
		t.Fatal("existing policy must not be recreated")
	}
}

func TestTenantProvisionerCreatesPolicyOnlyWhenRoleExists(t *testing.T) {
	repo := &fakeTenantRepository{existingRoleID: 100}
	provisioner := NewTenantProvisioner(repo)

	if err := provisioner.EnsureDefaults(context.Background(), 42); err != nil {
		t.Fatalf("EnsureDefaults() error = %v", err)
	}
	if repo.createdRole != nil {
		t.Fatal("existing role must not be recreated")
	}
	if repo.createdCasbinRule == nil {
		t.Fatal("expected casbin policy to be created when role exists but policy missing")
	}
	if repo.createdCasbinRule.V1 != "100" {
		t.Fatalf("policy should reference existing roleID=100, got v1=%s", repo.createdCasbinRule.V1)
	}
}

type fakeTenantRepository struct {
	existingRoleID    uint64
	existingPolicy    bool
	createdRole       *TenantRole
	createdCasbinRule *TenantCasbinRule
}

func (r *fakeTenantRepository) FindRoleByTenantAndCode(_ context.Context, _ uint64, _ string) (uint64, bool, error) {
	if r.existingRoleID > 0 {
		return r.existingRoleID, true, nil
	}
	return 0, false, nil
}

func (r *fakeTenantRepository) CreateRole(_ context.Context, role TenantRole) (uint64, error) {
	r.createdRole = &role
	return 999, nil
}

func (r *fakeTenantRepository) CasbinRuleExists(_ context.Context, _, _, _, _, _ string) (bool, error) {
	return r.existingPolicy, nil
}

func (r *fakeTenantRepository) CreateCasbinRule(_ context.Context, rule TenantCasbinRule) error {
	r.createdCasbinRule = &rule
	return nil
}

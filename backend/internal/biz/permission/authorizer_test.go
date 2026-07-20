package permission

import (
	"context"
	"testing"
)

func TestAuthorizerKeepsPoliciesInsideTenantDomain(t *testing.T) {
	authorizer, err := NewAuthorizer()
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	ctx := context.Background()
	if err := authorizer.AddRoleForMember(ctx, 10, 100, 200); err != nil {
		t.Fatalf("AddRoleForMember() error = %v", err)
	}
	if err := authorizer.GrantRole(ctx, 10, 200, "iam.user", "read"); err != nil {
		t.Fatalf("GrantRole() error = %v", err)
	}

	allowed, err := authorizer.Enforce(ctx, 10, 100, "iam.user", "read")
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if !allowed {
		t.Fatal("tenant member permission = false, want true")
	}

	allowed, err = authorizer.Enforce(ctx, 11, 100, "iam.user", "read")
	if err != nil {
		t.Fatalf("Enforce(other tenant) error = %v", err)
	}
	if allowed {
		t.Fatal("cross-tenant permission = true, want false")
	}
}

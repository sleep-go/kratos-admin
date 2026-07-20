package tenant

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := WithContext(context.Background(), Scope{
		TenantID: 200,
		UserID:   100,
		MemberID: 300,
	})

	scope, err := FromContext(ctx)
	if err != nil {
		t.Fatalf("FromContext() error = %v", err)
	}
	if scope.TenantID != 200 || scope.UserID != 100 || scope.MemberID != 300 {
		t.Fatalf("scope = %+v", scope)
	}
}

func TestFromContextRejectsMissingTenant(t *testing.T) {
	if _, err := FromContext(context.Background()); err == nil {
		t.Fatal("FromContext() error = nil, want missing tenant error")
	}
}

func TestPlatformContextAllowsZeroTenant(t *testing.T) {
	ctx := WithPlatformContext(context.Background(), 100)

	scope, err := FromContext(ctx)
	if err != nil {
		t.Fatalf("FromContext() error = %v", err)
	}
	if !scope.Platform || scope.TenantID != 0 || scope.UserID != 100 {
		t.Fatalf("scope = %+v", scope)
	}
}

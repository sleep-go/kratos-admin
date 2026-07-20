package model

import "testing"

func Test核心模型映射固定表名(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"租户", (Tenant{}).TableName(), "tenants"},
		{"用户", (User{}).TableName(), "users"},
		{"成员", (TenantMember{}).TableName(), "tenant_members"},
		{"会话", (AuthSession{}).TableName(), "auth_sessions"},
		{"审计Outbox", (AuditOutbox{}).TableName(), "audit_outbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("TableName() = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

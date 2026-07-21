package model

import (
	"reflect"
	"testing"

	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
)

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

func TestRabbitMQTaskStateFields(t *testing.T) {
	tests := []struct {
		model  any
		fields []string
	}{
		{model: AuditOutbox{}, fields: []string{"DispatchedAt", "LastError"}},
		{model: LogExport{}, fields: []string{"DispatchedAt"}},
		{model: File{}, fields: []string{"CleanupDispatchedAt", "CleanupRetryCount", "CleanupNextRetryAt", "CleanupFailureReason"}},
	}
	for _, test := range tests {
		typeOf := reflect.TypeOf(test.model)
		for _, field := range test.fields {
			if _, ok := typeOf.FieldByName(field); !ok {
				t.Errorf("%s missing field %s", typeOf.Name(), field)
			}
		}
	}
	if filebiz.StatusDeletionPending != 5 {
		t.Fatalf("StatusDeletionPending = %d", filebiz.StatusDeletionPending)
	}
}

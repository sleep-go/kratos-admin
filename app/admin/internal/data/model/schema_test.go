package model

import (
	"reflect"
	"testing"

	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
)

func Test核心模型映射固定表名(t *testing.T) {
	tenant := Tenant{}
	user := User{}
	member := TenantMember{}
	session := AuthSession{}
	outbox := AuditOutbox{}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"租户", tenant.TableName(), "tenants"},
		{"用户", user.TableName(), "users"},
		{"成员", member.TableName(), "tenant_members"},
		{"会话", session.TableName(), "auth_sessions"},
		{"审计Outbox", outbox.TableName(), "audit_outbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("TableName() = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestGeneratedModelCoreTypes(t *testing.T) {
	userType := reflect.TypeOf(User{})
	id, ok := userType.FieldByName("ID")
	if !ok || id.Type.Kind() != reflect.Uint64 {
		t.Fatal("User.ID 必须为 uint64")
	}
	platformAdmin, ok := userType.FieldByName("IsPlatformAdmin")
	if !ok || platformAdmin.Type.Kind() != reflect.Bool {
		t.Fatal("User.IsPlatformAdmin 必须为 bool")
	}
	status, ok := userType.FieldByName("Status")
	if !ok || status.Type.Kind() != reflect.Uint8 {
		t.Fatal("User.Status 必须为 uint8")
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

package model

import (
	"reflect"
	"testing"

	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
)

func Test核心模型映射固定表名(t *testing.T) {
	tenant := Tenant{}
	appUser := AppUser{}
	tenantAdmin := TenantAdmin{}
	session := AuthSession{}
	outbox := AuditOutbox{}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"租户", tenant.TableName(), "tenants"},
		{"App用户", appUser.TableName(), "app_users"},
		{"租户管理员", tenantAdmin.TableName(), "tenant_admins"},
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
	appUserType := reflect.TypeOf(AppUser{})
	id, ok := appUserType.FieldByName("ID")
	if !ok || id.Type.Kind() != reflect.Uint64 {
		t.Fatal("AppUser.ID 必须为 uint64")
	}
	status, ok := appUserType.FieldByName("Status")
	if !ok || status.Type.Kind() != reflect.Uint8 {
		t.Fatal("AppUser.Status 必须为 uint8")
	}
}

func TestResourceScopeMaskGenerated(t *testing.T) {
	field, ok := reflect.TypeOf(Resource{}).FieldByName("ScopeMask")
	if !ok || field.Type.Kind() != reflect.Uint8 {
		t.Fatal("Resource.ScopeMask 必须生成为 uint8")
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

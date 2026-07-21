package service

import (
	"context"
	"testing"
	"time"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/storage"
)

type fakeLogHandler struct{ access logexport.Access }

func (h *fakeLogHandler) Create(_ context.Context, access logexport.Access, logType, _ string, _ map[string]string) (logexport.Record, error) {
	h.access = access
	return logexport.Record{ID: "export-1", LogType: logType, Status: logexport.StatusPending, CreatedAt: time.Now()}, nil
}
func (h *fakeLogHandler) Get(context.Context, logexport.Access, string) (logexport.Record, error) {
	return logexport.Record{}, nil
}
func (h *fakeLogHandler) DownloadURL(context.Context, logexport.Access, string) (storage.SignedRequest, error) {
	return storage.SignedRequest{}, nil
}

func TestPlatformAdministratorInTenantContextDoesNotBypassLogPermission(t *testing.T) {
	handler := &fakeLogHandler{}
	service := NewLogService(handler, fakePermissionChecker{allowed: false})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 1, TenantID: 8, MemberID: 9, Realm: bizauth.RealmTenant, ImpersonatorID: 1})
	if _, err := service.CreateExport(ctx, &v1.CreateExportRequest{LogType: "api"}); err == nil {
		t.Fatal("平台管理员在租户上下文必须接受日志导出权限检查")
	}
	if handler.access.PlatformAdmin {
		t.Fatalf("access = %+v", handler.access)
	}
}

func TestPlatformAdministratorInPlatformContextCanExportLogs(t *testing.T) {
	handler := &fakeLogHandler{}
	service := NewLogService(handler, fakePermissionChecker{allowed: false})
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 1, TenantID: 0, Realm: bizauth.RealmPlatform})
	reply, err := service.CreateExport(ctx, &v1.CreateExportRequest{LogType: "api"})
	if err != nil || reply.Item.GetId() != "export-1" || !handler.access.PlatformAdmin {
		t.Fatalf("CreateExport() = %+v, %v, access %+v", reply, err, handler.access)
	}
}

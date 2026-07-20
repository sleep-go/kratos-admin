package service

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/internal/biz/logexport"
	managementbiz "github.com/sleep-go/kratos-admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

// LogExportHandler 定义日志导出创建、查询和下载能力。
type LogExportHandler interface {
	Create(ctx context.Context, access logexport.Access, logType, keyword string, filters map[string]string) (logexport.Record, error)
	Get(ctx context.Context, access logexport.Access, exportID string) (logexport.Record, error)
	DownloadURL(ctx context.Context, access logexport.Access, exportID string) (storage.SignedRequest, error)
}

// LogService 实现受权限保护的异步日志导出 API。
type LogService struct {
	v1.UnimplementedLogServiceServer
	handler     LogExportHandler
	permissions managementbiz.PermissionChecker
}

// NewLogService 创建日志导出服务。
func NewLogService(handler LogExportHandler, checkers ...managementbiz.PermissionChecker) *LogService {
	service := &LogService{handler: handler}
	if len(checkers) > 0 {
		service.permissions = checkers[0]
	}
	return service
}

// CreateExport 创建最多包含十万条记录的异步导出任务。
func (s *LogService) CreateExport(ctx context.Context, request *v1.CreateExportRequest) (*v1.CreateExportResponse, error) {
	access, err := s.access(ctx)
	if err != nil {
		return nil, err
	}
	resource := request.GetLogType() + "-logs"
	if !access.PlatformAdmin && s.permissions != nil {
		allowed, checkErr := s.permissions.Allowed(ctx, managementbiz.Scope{
			TenantID: access.TenantID, UserID: access.UserID, MemberID: access.MemberID,
		}, resource, "export")
		if checkErr != nil {
			return nil, kratoserrors.InternalServer("PERMISSION_CHECK_FAILED", "权限校验失败")
		}
		if !allowed {
			return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "没有导出该日志的权限")
		}
	}
	record, err := s.handler.Create(ctx, access, request.GetLogType(), request.GetKeyword(), request.GetFilters())
	if err != nil {
		return nil, mapLogExportError(err)
	}
	return &v1.CreateExportResponse{Item: mapLogExport(record)}, nil
}

// GetExport 返回当前用户可见的导出任务状态。
func (s *LogService) GetExport(ctx context.Context, request *v1.GetExportRequest) (*v1.GetExportResponse, error) {
	access, err := s.access(ctx)
	if err != nil {
		return nil, err
	}
	record, err := s.handler.Get(ctx, access, request.GetExportId())
	if err != nil {
		return nil, mapLogExportError(err)
	}
	return &v1.GetExportResponse{Item: mapLogExport(record)}, nil
}

// GetExportDownloadURL 重新校验任务归属后签发五分钟下载地址。
func (s *LogService) GetExportDownloadURL(ctx context.Context, request *v1.GetExportDownloadURLRequest) (*v1.GetExportDownloadURLResponse, error) {
	access, err := s.access(ctx)
	if err != nil {
		return nil, err
	}
	signed, err := s.handler.DownloadURL(ctx, access, request.GetExportId())
	if err != nil {
		return nil, mapLogExportError(err)
	}
	return &v1.GetExportDownloadURLResponse{Url: signed.URL, ExpiresAt: timestamppb.New(signed.ExpiresAt)}, nil
}

func (s *LogService) access(ctx context.Context) (logexport.Access, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok {
		return logexport.Access{}, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	return logexport.Access{
		TenantID: claims.TenantID, UserID: claims.UserID, MemberID: claims.MemberID, PlatformAdmin: claims.PlatformAdmin,
	}, nil
}

func mapLogExport(record logexport.Record) *v1.LogExport {
	result := &v1.LogExport{
		Id: record.ID, LogType: record.LogType, Status: uint32(record.Status), RowCount: record.RowCount,
		FileId: record.FileID, RetryCount: record.RetryCount, FailureReason: record.FailureReason,
		CreatedAt: timestamppb.New(record.CreatedAt),
	}
	if record.FinishedAt != nil {
		result.FinishedAt = timestamppb.New(*record.FinishedAt)
	}
	return result
}

func mapLogExportError(err error) error {
	switch {
	case errors.Is(err, logexport.ErrInvalidRequest):
		return kratoserrors.BadRequest("LOG_EXPORT_INVALID", err.Error())
	case errors.Is(err, logexport.ErrNotFound):
		return kratoserrors.NotFound("LOG_EXPORT_NOT_FOUND", err.Error())
	case errors.Is(err, logexport.ErrNotReady):
		return kratoserrors.Conflict("LOG_EXPORT_NOT_READY", err.Error())
	default:
		return kratoserrors.InternalServer("LOG_EXPORT_FAILED", "日志导出服务暂时不可用")
	}
}

var _ v1.LogServiceServer = (*LogService)(nil)

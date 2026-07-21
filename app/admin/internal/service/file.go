package service

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/storage"
)

// FileHandler 定义文件上传、确认、下载和删除用例。
type FileHandler interface {
	CreateUpload(ctx context.Context, input filebiz.UploadInput) (filebiz.UploadResult, error)
	ConfirmUpload(ctx context.Context, tenantID uint64, fileID string) (filebiz.Record, error)
	DownloadURL(ctx context.Context, tenantID uint64, fileID string) (storage.SignedRequest, error)
	Delete(ctx context.Context, tenantID uint64, fileID string) error
	AddReference(ctx context.Context, tenantID uint64, fileID, businessType, businessID string) error
	RemoveReference(ctx context.Context, tenantID uint64, fileID, businessType, businessID string) error
}

// FileService 实现租户文件 API。
type FileService struct {
	v1.UnimplementedFileServiceServer
	handler     FileHandler
	permissions managementbiz.PermissionChecker
	dataScope   managementbiz.RecordChecker
}

// NewFileService 创建文件服务。
func NewFileService(handler FileHandler, checkers ...managementbiz.PermissionChecker) *FileService {
	service := &FileService{handler: handler}
	if len(checkers) > 0 {
		service.permissions = checkers[0]
		if dataScope, ok := checkers[0].(managementbiz.RecordChecker); ok {
			service.dataScope = dataScope
		}
	}
	return service
}

// CreateUpload 预登记文件并返回短期上传请求。
func (s *FileService) CreateUpload(ctx context.Context, request *v1.CreateUploadRequest) (*v1.CreateUploadResponse, error) {
	scope, err := s.fileScope(ctx, "create", "")
	if err != nil {
		return nil, err
	}
	result, err := s.handler.CreateUpload(ctx, filebiz.UploadInput{
		TenantID: scope.TenantID, MemberID: scope.MemberID, OriginalName: request.GetOriginalName(),
		ContentType: request.GetContentType(), Size: int64(request.GetSizeBytes()), SHA256: request.GetSha256(),
	})
	if err != nil {
		return nil, mapFileError(err)
	}
	return &v1.CreateUploadResponse{
		FileId: result.Record.ID, ProviderName: result.Record.ProviderName, ObjectKey: result.Record.ObjectKey,
		Upload: mapSignedFileRequest(result.Upload),
	}, nil
}

// ConfirmUpload 校验对象元数据并将文件标记为可用。
func (s *FileService) ConfirmUpload(ctx context.Context, request *v1.ConfirmUploadRequest) (*v1.ConfirmUploadResponse, error) {
	scope, err := s.fileScope(ctx, "update", request.GetFileId())
	if err != nil {
		return nil, err
	}
	record, err := s.handler.ConfirmUpload(ctx, scope.TenantID, request.GetFileId())
	if err != nil {
		return nil, mapFileError(err)
	}
	return mapFileResponse(record), nil
}

// GetDownloadURL 重新校验租户权限和文件状态后返回短期下载请求。
func (s *FileService) GetDownloadURL(ctx context.Context, request *v1.GetDownloadURLRequest) (*v1.GetDownloadURLResponse, error) {
	scope, err := s.fileScope(ctx, "download", request.GetFileId())
	if err != nil {
		return nil, err
	}
	signed, err := s.handler.DownloadURL(ctx, scope.TenantID, request.GetFileId())
	if err != nil {
		return nil, mapFileError(err)
	}
	return &v1.GetDownloadURLResponse{Download: mapSignedFileRequest(signed)}, nil
}

// DeleteFile 请求后台异步清理无业务引用的文件对象。
func (s *FileService) DeleteFile(ctx context.Context, request *v1.DeleteFileRequest) (*v1.DeleteFileResponse, error) {
	scope, err := s.fileScope(ctx, "delete", request.GetFileId())
	if err != nil {
		return nil, err
	}
	if err := s.handler.Delete(ctx, scope.TenantID, request.GetFileId()); err != nil {
		return nil, mapFileError(err)
	}
	return &v1.DeleteFileResponse{}, nil
}

// AddReference 绑定文件与租户内业务资源，阻止文件被提前删除。
func (s *FileService) AddReference(ctx context.Context, request *v1.AddReferenceRequest) (*v1.AddReferenceResponse, error) {
	scope, err := s.fileScope(ctx, "update", request.GetFileId())
	if err != nil {
		return nil, err
	}
	if err := s.handler.AddReference(ctx, scope.TenantID, request.GetFileId(), request.GetBusinessType(), request.GetBusinessId()); err != nil {
		return nil, mapFileError(err)
	}
	return &v1.AddReferenceResponse{}, nil
}

// RemoveReference 解除文件与租户内业务资源的引用关系。
func (s *FileService) RemoveReference(ctx context.Context, request *v1.RemoveReferenceRequest) (*v1.RemoveReferenceResponse, error) {
	scope, err := s.fileScope(ctx, "update", request.GetFileId())
	if err != nil {
		return nil, err
	}
	if err := s.handler.RemoveReference(ctx, scope.TenantID, request.GetFileId(), request.GetBusinessType(), request.GetBusinessId()); err != nil {
		return nil, mapFileError(err)
	}
	return &v1.RemoveReferenceResponse{}, nil
}

func (s *FileService) fileScope(ctx context.Context, action, fileID string) (managementbiz.Scope, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || claims.TenantID == 0 {
		return managementbiz.Scope{}, kratoserrors.Unauthorized("TENANT_AUTH_REQUIRED", "请先进入租户")
	}
	impersonating := claims.ImpersonatorID > 0
	if !impersonating && claims.MemberID == 0 {
		return managementbiz.Scope{}, kratoserrors.Unauthorized("TENANT_AUTH_REQUIRED", "请先进入租户")
	}
	scope := managementbiz.Scope{
		TenantID: claims.TenantID, UserID: claims.UserID, MemberID: claims.MemberID,
		Impersonating: impersonating,
	}
	if s.permissions != nil {
		allowed, err := s.permissions.Allowed(ctx, scope, "files", action)
		if err != nil {
			return managementbiz.Scope{}, kratoserrors.InternalServer("PERMISSION_CHECK_FAILED", "权限校验失败")
		}
		if !allowed {
			return managementbiz.Scope{}, kratoserrors.Forbidden("PERMISSION_DENIED", "没有执行该操作的权限")
		}
	}
	if fileID != "" && s.dataScope != nil {
		allowed, err := s.dataScope.AllowedRecord(ctx, scope, "files", fileID)
		if err != nil {
			return managementbiz.Scope{}, kratoserrors.InternalServer("DATA_SCOPE_CHECK_FAILED", "数据范围校验失败")
		}
		if !allowed {
			return managementbiz.Scope{}, kratoserrors.Forbidden("DATA_SCOPE_DENIED", "文件超出当前角色数据范围")
		}
	}
	return scope, nil
}

func mapSignedFileRequest(signed storage.SignedRequest) *v1.SignedFileRequest {
	return &v1.SignedFileRequest{
		Method: signed.Method, Url: signed.URL, Headers: signed.Headers, ExpiresAt: timestamppb.New(signed.ExpiresAt),
	}
}

func mapFileResponse(record filebiz.Record) *v1.ConfirmUploadResponse {
	return &v1.ConfirmUploadResponse{
		Id: record.ID, ProviderName: record.ProviderName, ObjectKey: record.ObjectKey,
		OriginalName: record.OriginalName, ContentType: record.ContentType, SizeBytes: uint64(record.Size),
		Sha256: record.SHA256, Status: uint32(record.Status), CreatedAt: timestamppb.New(record.CreatedAt),
	}
}

func mapFileError(err error) error {
	switch {
	case errors.Is(err, filebiz.ErrInvalidFile):
		return kratoserrors.BadRequest("FILE_INVALID", err.Error())
	case errors.Is(err, filebiz.ErrObjectMismatch):
		return kratoserrors.BadRequest("FILE_OBJECT_MISMATCH", err.Error())
	case errors.Is(err, filebiz.ErrFileUnavailable):
		return kratoserrors.NotFound("FILE_NOT_FOUND", err.Error())
	case errors.Is(err, filebiz.ErrFileReferenced):
		return kratoserrors.Conflict("FILE_STILL_REFERENCED", err.Error())
	default:
		return kratoserrors.InternalServer("FILE_PROVIDER_FAILED", "文件服务暂时不可用")
	}
}

var _ v1.FileServiceServer = (*FileService)(nil)

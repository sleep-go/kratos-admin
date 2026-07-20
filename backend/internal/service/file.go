package service

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
	filebiz "github.com/sleep-go/kratos-admin/backend/internal/biz/file"
	"github.com/sleep-go/kratos-admin/backend/internal/provider/storage"
)

// FileHandler 定义文件上传、确认、下载和删除用例。
type FileHandler interface {
	CreateUpload(ctx context.Context, input filebiz.UploadInput) (filebiz.UploadResult, error)
	ConfirmUpload(ctx context.Context, tenantID uint64, fileID string) (filebiz.Record, error)
	DownloadURL(ctx context.Context, tenantID uint64, fileID string) (storage.SignedRequest, error)
	Delete(ctx context.Context, tenantID uint64, fileID string) error
}

// FileService 实现租户文件 API。
type FileService struct {
	v1.UnimplementedFileServiceServer
	handler     FileHandler
	permissions ManagementPermissionChecker
}

// NewFileService 创建文件服务。
func NewFileService(handler FileHandler, checkers ...ManagementPermissionChecker) *FileService {
	service := &FileService{handler: handler}
	if len(checkers) > 0 {
		service.permissions = checkers[0]
	}
	return service
}

// CreateUpload 预登记文件并返回短期上传请求。
func (s *FileService) CreateUpload(ctx context.Context, request *v1.CreateUploadRequest) (*v1.CreateUploadResponse, error) {
	scope, err := s.fileScope(ctx, "create")
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
	scope, err := s.fileScope(ctx, "update")
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
	scope, err := s.fileScope(ctx, "download")
	if err != nil {
		return nil, err
	}
	signed, err := s.handler.DownloadURL(ctx, scope.TenantID, request.GetFileId())
	if err != nil {
		return nil, mapFileError(err)
	}
	return &v1.GetDownloadURLResponse{Download: mapSignedFileRequest(signed)}, nil
}

// DeleteFile 删除对象并逻辑删除文件元数据。
func (s *FileService) DeleteFile(ctx context.Context, request *v1.DeleteFileRequest) (*v1.DeleteFileResponse, error) {
	scope, err := s.fileScope(ctx, "delete")
	if err != nil {
		return nil, err
	}
	if err := s.handler.Delete(ctx, scope.TenantID, request.GetFileId()); err != nil {
		return nil, mapFileError(err)
	}
	return &v1.DeleteFileResponse{}, nil
}

func (s *FileService) fileScope(ctx context.Context, action string) (ResourceScope, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || claims.TenantID == 0 || claims.MemberID == 0 {
		return ResourceScope{}, kratoserrors.Unauthorized("TENANT_AUTH_REQUIRED", "请先进入租户")
	}
	scope := ResourceScope{TenantID: claims.TenantID, UserID: claims.UserID, MemberID: claims.MemberID}
	if s.permissions != nil {
		allowed, err := s.permissions.Allowed(ctx, scope, "files", action)
		if err != nil {
			return ResourceScope{}, kratoserrors.InternalServer("PERMISSION_CHECK_FAILED", "权限校验失败")
		}
		if !allowed {
			return ResourceScope{}, kratoserrors.Forbidden("PERMISSION_DENIED", "没有执行该操作的权限")
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
	default:
		return kratoserrors.InternalServer("FILE_PROVIDER_FAILED", "文件服务暂时不可用")
	}
}

var _ v1.FileServiceServer = (*FileService)(nil)

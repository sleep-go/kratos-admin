package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	filebiz "github.com/sleep-go/kratos-admin/backend/internal/biz/file"
	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

// FileRepository 使用 MySQL 持久化租户文件元数据和审计 Outbox。
type FileRepository struct {
	db *gorm.DB
}

// NewFileRepository 创建文件仓储。
func NewFileRepository(data *Data) *FileRepository {
	return &FileRepository{db: data.DB}
}

// Create 在同一事务中预登记文件并写入审计 Outbox。
func (r *FileRepository) Create(ctx context.Context, record filebiz.Record) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := &model.File{
			ID: record.ID, TenantID: record.TenantID, UploaderMemberID: record.UploaderMemberID,
			ProviderName: record.ProviderName, ObjectKey: record.ObjectKey, OriginalName: record.OriginalName,
			ContentType: record.ContentType, SizeBytes: uint64(record.Size), SHA256: record.SHA256,
			Status: record.Status, CreatedAt: record.CreatedAt, UpdatedAt: record.CreatedAt,
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return writeAuditOutbox(tx, service.ResourceScope{
			TenantID: record.TenantID, MemberID: record.UploaderMemberID,
		}, "create-upload", "files", record.ID, map[string]any{
			"provider_name": record.ProviderName, "object_key": record.ObjectKey,
			"original_name": record.OriginalName, "content_type": record.ContentType, "size_bytes": record.Size,
		})
	})
}

// Find 按认证租户查询未逻辑删除的文件。
func (r *FileRepository) Find(ctx context.Context, tenantID uint64, fileID string) (filebiz.Record, error) {
	var row model.File
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", fileID, tenantID).Take(&row).Error
	if err != nil {
		return filebiz.Record{}, err
	}
	return filebiz.Record{
		ID: row.ID, TenantID: row.TenantID, UploaderMemberID: row.UploaderMemberID,
		ProviderName: row.ProviderName, ObjectKey: row.ObjectKey, OriginalName: row.OriginalName,
		ContentType: row.ContentType, Size: int64(row.SizeBytes), SHA256: row.SHA256, ETag: row.ETag,
		Status: row.Status, CreatedAt: row.CreatedAt,
	}, nil
}

// Confirm 将待确认文件标记为可用并记录对象 ETag。
func (r *FileRepository) Confirm(ctx context.Context, tenantID uint64, fileID, etag string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("files").Where("id = ? AND tenant_id = ? AND status = ? AND deleted_at IS NULL", fileID, tenantID, filebiz.StatusPending).
			Updates(map[string]any{"status": filebiz.StatusAvailable, "etag": etag, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("待确认文件不存在")
		}
		return writeAuditOutbox(tx, service.ResourceScope{TenantID: tenantID}, "confirm", "files", fileID, map[string]any{"status": filebiz.StatusAvailable})
	})
}

// MarkDeleted 逻辑删除文件元数据并写入审计 Outbox。
func (r *FileRepository) MarkDeleted(ctx context.Context, tenantID uint64, fileID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Table("files").Where("id = ? AND tenant_id = ? AND status <> ? AND deleted_at IS NULL", fileID, tenantID, filebiz.StatusDeleted).
			Updates(map[string]any{"status": filebiz.StatusDeleted, "deleted_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("文件不存在或已删除")
		}
		return writeAuditOutbox(tx, service.ResourceScope{TenantID: tenantID}, "delete", "files", fileID, nil)
	})
}

var _ filebiz.Repository = (*FileRepository)(nil)

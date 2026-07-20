package data

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// RequestDelete 检查业务引用后冻结文件，等待 Worker 异步清理对象。
func (r *FileRepository) RequestDelete(ctx context.Context, tenantID uint64, fileID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.File
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ? AND status = ? AND deleted_at IS NULL", fileID, tenantID, filebiz.StatusAvailable).Take(&row).Error; err != nil {
			return filebiz.ErrFileUnavailable
		}
		var referenceCount int64
		if err := tx.Model(&model.FileReference{}).Where("tenant_id = ? AND file_id = ?", tenantID, fileID).Count(&referenceCount).Error; err != nil {
			return err
		}
		if referenceCount > 0 {
			return filebiz.ErrFileReferenced
		}
		result := tx.Table("files").Where("id = ? AND tenant_id = ? AND status = ?", fileID, tenantID, filebiz.StatusAvailable).
			Updates(map[string]any{"status": filebiz.StatusDeletionPending, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return filebiz.ErrFileUnavailable
		}
		return writeAuditOutbox(tx, service.ResourceScope{TenantID: tenantID}, "request-delete", "files", fileID, nil)
	})
}

// AddReference 在锁定可用文件后新增业务引用，避免与删除请求竞态。
func (r *FileRepository) AddReference(ctx context.Context, tenantID uint64, fileID, businessType, businessID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.File
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ? AND status = ? AND deleted_at IS NULL", fileID, tenantID, filebiz.StatusAvailable).Take(&row).Error; err != nil {
			return filebiz.ErrFileUnavailable
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.FileReference{
			TenantID: tenantID, FileID: fileID, BusinessType: businessType, BusinessID: businessID, CreatedAt: time.Now().UTC(),
		}).Error
	})
}

// RemoveReference 删除指定租户业务资源与文件的引用关系。
func (r *FileRepository) RemoveReference(ctx context.Context, tenantID uint64, fileID, businessType, businessID string) error {
	result := r.db.WithContext(ctx).Where("tenant_id = ? AND file_id = ? AND business_type = ? AND business_id = ?", tenantID, fileID, businessType, businessID).
		Delete(&model.FileReference{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("文件引用不存在")
	}
	return nil
}

// PendingCleanup 返回等待 Worker 删除对象的文件。
func (r *FileRepository) PendingCleanup(ctx context.Context, limit int) ([]filebiz.Record, error) {
	var rows []model.File
	if err := r.db.WithContext(ctx).Where("status = ? AND deleted_at IS NULL", filebiz.StatusDeletionPending).Order("updated_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]filebiz.Record, 0, len(rows))
	for _, row := range rows {
		result = append(result, filebiz.Record{ID: row.ID, TenantID: row.TenantID, ProviderName: row.ProviderName, ObjectKey: row.ObjectKey, Status: row.Status})
	}
	return result, nil
}

// CompleteCleanup 在对象删除成功后逻辑删除文件元数据。
func (r *FileRepository) CompleteCleanup(ctx context.Context, tenantID uint64, fileID string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Table("files").Where("id = ? AND tenant_id = ? AND status = ? AND deleted_at IS NULL", fileID, tenantID, filebiz.StatusDeletionPending).
		Updates(map[string]any{"status": filebiz.StatusDeleted, "deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return filebiz.ErrFileUnavailable
	}
	return nil
}

// FailCleanup 在超过最大重试次数后记录对象清理失败状态。
func (r *FileRepository) FailCleanup(ctx context.Context, tenantID uint64, fileID string) error {
	return r.db.WithContext(ctx).Table("files").Where("id = ? AND tenant_id = ? AND status = ? AND deleted_at IS NULL", fileID, tenantID, filebiz.StatusDeletionPending).
		Updates(map[string]any{"status": filebiz.StatusCleanupFailed, "updated_at": time.Now().UTC()}).Error
}

var _ filebiz.Repository = (*FileRepository)(nil)

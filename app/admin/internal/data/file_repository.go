package data

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

// FileRepository 使用 MySQL 持久化租户文件元数据和审计 Outbox。
type FileRepository struct {
	q *query.Query
}

// NewFileRepository 创建文件仓储。
func NewFileRepository(data *Data) *FileRepository {
	return &FileRepository{q: data.Query}
}

// Create 在同一事务中预登记文件并写入审计 Outbox。
func (r *FileRepository) Create(ctx context.Context, record filebiz.Record) error {
	return r.q.Transaction(func(tx *query.Query) error {
		row := &model.File{
			ID: record.ID, TenantID: record.TenantID, UploaderID: record.UploaderID,
			ProviderName: record.ProviderName, ObjectKey: record.ObjectKey, OriginalName: record.OriginalName,
			ContentType: record.ContentType, SizeBytes: uint64(record.Size), SHA256: record.SHA256,
			Status: record.Status, CreatedAt: record.CreatedAt, UpdatedAt: record.CreatedAt,
		}
		if err := tx.File.WithContext(ctx).Create(row); err != nil {
			return err
		}
		return writeAuditOutboxGen(ctx, tx, managementbiz.Scope{
			TenantID: record.TenantID, MemberID: record.UploaderID,
		}, "create-upload", "files", record.ID, map[string]any{
			"provider_name": record.ProviderName, "object_key": record.ObjectKey,
			"original_name": record.OriginalName, "content_type": record.ContentType, "size_bytes": record.Size,
		})
	})
}

// Find 按认证租户查询未逻辑删除的文件。
func (r *FileRepository) Find(ctx context.Context, tenantID uint64, fileID string) (filebiz.Record, error) {
	f := r.q.File
	row, err := f.WithContext(ctx).Where(f.ID.Eq(fileID), f.TenantID.Eq(tenantID), f.DeletedAt.IsNull()).Take()
	if err != nil {
		return filebiz.Record{}, err
	}
	return filebiz.Record{
		ID: row.ID, TenantID: row.TenantID, UploaderID: row.UploaderID,
		ProviderName: row.ProviderName, ObjectKey: row.ObjectKey, OriginalName: row.OriginalName,
		ContentType: row.ContentType, Size: int64(row.SizeBytes), SHA256: row.SHA256, ETag: row.ETag,
		Status: row.Status, CreatedAt: row.CreatedAt,
	}, nil
}

// Confirm 将待确认文件标记为可用并记录对象 ETag。
func (r *FileRepository) Confirm(ctx context.Context, tenantID uint64, fileID, etag string) error {
	return r.q.Transaction(func(tx *query.Query) error {
		f := tx.File
		result, err := f.WithContext(ctx).
			Where(f.ID.Eq(fileID), f.TenantID.Eq(tenantID), f.Status.Eq(filebiz.StatusPending), f.DeletedAt.IsNull()).
			UpdateSimple(f.Status.Value(filebiz.StatusAvailable), f.ETag.Value(etag), f.UpdatedAt.Value(time.Now().UTC()))
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return errors.New("待确认文件不存在")
		}
		return writeAuditOutboxGen(ctx, tx, managementbiz.Scope{TenantID: tenantID}, "confirm", "files", fileID, map[string]any{"status": filebiz.StatusAvailable})
	})
}

// RequestDelete 检查业务引用后冻结文件，等待后台异步清理对象。
func (r *FileRepository) RequestDelete(ctx context.Context, tenantID uint64, fileID string) error {
	return r.q.Transaction(func(tx *query.Query) error {
		f := tx.File
		_, err := f.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(f.ID.Eq(fileID), f.TenantID.Eq(tenantID), f.Status.Eq(filebiz.StatusAvailable), f.DeletedAt.IsNull()).Take()
		if err != nil {
			return filebiz.ErrFileUnavailable
		}
		reference := tx.FileReference
		referenceCount, err := reference.WithContext(ctx).Where(reference.TenantID.Eq(tenantID), reference.FileID.Eq(fileID)).Count()
		if err != nil {
			return err
		}
		if referenceCount > 0 {
			return filebiz.ErrFileReferenced
		}
		result, err := f.WithContext(ctx).Where(f.ID.Eq(fileID), f.TenantID.Eq(tenantID), f.Status.Eq(filebiz.StatusAvailable)).
			UpdateSimple(f.Status.Value(filebiz.StatusDeletionPending), f.UpdatedAt.Value(time.Now().UTC()))
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return filebiz.ErrFileUnavailable
		}
		return writeAuditOutboxGen(ctx, tx, managementbiz.Scope{TenantID: tenantID}, "request-delete", "files", fileID, nil)
	})
}

// AddReference 在锁定可用文件后新增业务引用，避免与删除请求竞态。
func (r *FileRepository) AddReference(ctx context.Context, tenantID uint64, fileID, businessType, businessID string) error {
	return r.q.Transaction(func(tx *query.Query) error {
		f := tx.File
		if _, err := f.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(f.ID.Eq(fileID), f.TenantID.Eq(tenantID), f.Status.Eq(filebiz.StatusAvailable), f.DeletedAt.IsNull()).Take(); err != nil {
			return filebiz.ErrFileUnavailable
		}
		return tx.FileReference.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.FileReference{
			TenantID: tenantID, FileID: fileID, BusinessType: businessType, BusinessID: businessID, CreatedAt: time.Now().UTC(),
		})
	})
}

// RemoveReference 删除指定租户业务资源与文件的引用关系。
func (r *FileRepository) RemoveReference(ctx context.Context, tenantID uint64, fileID, businessType, businessID string) error {
	reference := r.q.FileReference
	result, err := reference.WithContext(ctx).
		Where(reference.TenantID.Eq(tenantID), reference.FileID.Eq(fileID), reference.BusinessType.Eq(businessType), reference.BusinessID.Eq(businessID)).
		Delete()
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errors.New("文件引用不存在")
	}
	return nil
}

// CompleteCleanup 在对象删除成功后逻辑删除文件元数据。
func (r *FileRepository) CompleteCleanup(ctx context.Context, tenantID uint64, fileID string) error {
	now := time.Now().UTC()
	f := r.q.File
	result, err := f.WithContext(ctx).
		Where(f.ID.Eq(fileID), f.TenantID.Eq(tenantID), f.Status.Eq(filebiz.StatusDeletionPending), f.DeletedAt.IsNull()).
		UpdateSimple(f.Status.Value(filebiz.StatusDeleted), f.DeletedAt.Value(gorm.DeletedAt{Time: now, Valid: true}), f.UpdatedAt.Value(now))
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return filebiz.ErrFileUnavailable
	}
	return nil
}

var _ filebiz.Repository = (*FileRepository)(nil)

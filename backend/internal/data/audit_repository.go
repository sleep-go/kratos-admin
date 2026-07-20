package data

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sleep-go/kratos-admin/backend/internal/biz/audit"
	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
)

// AuditRepository 实现事务 Outbox 的查询和幂等审计落库。
type AuditRepository struct{ db *gorm.DB }

// NewAuditRepository 创建审计 Outbox 仓储。
func NewAuditRepository(data *Data) *AuditRepository { return &AuditRepository{db: data.DB} }

// Get 读取指定 Outbox 事件。
func (r *AuditRepository) Get(ctx context.Context, eventID string) (audit.Event, error) {
	var row model.AuditOutbox
	err := r.db.WithContext(ctx).Where("id = ?", eventID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return audit.Event{}, audit.ErrEventNotFound
	}
	return audit.Event{ID: row.ID, TenantID: row.TenantID, Payload: []byte(row.Payload)}, err
}

// Publish 在一个事务中幂等写审计并标记 Outbox 已发布。
func (r *AuditRepository) Publish(ctx context.Context, event audit.Event, entry audit.Entry) (bool, error) {
	published := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var outbox model.AuditOutbox
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", event.ID).First(&outbox).Error; err != nil {
			return err
		}
		if outbox.Status == 2 {
			return nil
		}
		before, err := json.Marshal(entry.Before)
		if err != nil {
			return err
		}
		after, err := json.Marshal(entry.After)
		if err != nil {
			return err
		}
		logRow := &model.AuditLog{
			EventID: event.ID, TenantID: event.TenantID, UserID: entry.UserID, MemberID: entry.MemberID,
			Action: entry.Action, ResourceType: entry.ResourceType, ResourceID: entry.ResourceID, Summary: entry.Summary,
			BeforeData: datatypes.JSON(before), AfterData: datatypes.JSON(after), IP: entry.IP,
			UserAgent: entry.UserAgent, RequestID: entry.RequestID,
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).Create(logRow).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&model.AuditOutbox{}).Where("id = ?", event.ID).Updates(map[string]any{"status": 2, "published_at": now}).Error; err != nil {
			return err
		}
		published = true
		return nil
	})
	return published, err
}

// PendingEventIDs 返回到期且尚未发布的 Outbox 事件 ID。
func (r *AuditRepository) PendingEventIDs(ctx context.Context, limit int) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.AuditOutbox{}).
		Where("status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", []int{1, 3}, time.Now().UTC()).
		Order("created_at ASC").Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

var _ audit.Repository = (*AuditRepository)(nil)

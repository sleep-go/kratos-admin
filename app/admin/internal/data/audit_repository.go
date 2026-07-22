package data

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/audit"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

// AuditRepository 实现事务 Outbox 的查询和幂等审计落库。
type AuditRepository struct{ q *query.Query }

// NewAuditRepository 创建审计 Outbox 仓储。
func NewAuditRepository(data *Data) *AuditRepository { return &AuditRepository{q: data.Query} }

// Get 读取指定 Outbox 事件。
func (r *AuditRepository) Get(ctx context.Context, eventID string) (audit.Event, error) {
	o := r.q.AuditOutbox
	row, err := o.WithContext(ctx).Where(o.ID.Eq(eventID)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return audit.Event{}, audit.ErrEventNotFound
	}
	return audit.Event{ID: row.ID, TenantID: row.TenantID, Payload: []byte(row.Payload)}, err
}

// Publish 在一个事务中幂等写审计并标记 Outbox 已发布。
func (r *AuditRepository) Publish(ctx context.Context, event audit.Event, entry audit.Entry) (bool, error) {
	published := false
	err := r.q.Transaction(func(tx *query.Query) error {
		o := tx.AuditOutbox
		outbox, err := o.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(o.ID.Eq(event.ID)).First()
		if err != nil {
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
			EventID: event.ID, TenantID: event.TenantID, Realm: entry.Realm, UserID: entry.UserID, MemberID: entry.MemberID,
			ImpersonatorID: entry.ImpersonatorID,
			Action: entry.Action, ResourceType: entry.ResourceType, ResourceID: entry.ResourceID, Summary: entry.Summary,
			BeforeData: datatypes.JSON(before), AfterData: datatypes.JSON(after), IP: entry.IP,
			UserAgent: entry.UserAgent, RequestID: entry.RequestID,
		}
		if err := tx.AuditLog.WithContext(ctx).
			Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).Create(logRow); err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := o.WithContext(ctx).Where(o.ID.Eq(event.ID)).UpdateSimple(o.Status.Value(2), o.PublishedAt.Value(now)); err != nil {
			return err
		}
		published = true
		return nil
	})
	return published, err
}

var _ audit.Repository = (*AuditRepository)(nil)

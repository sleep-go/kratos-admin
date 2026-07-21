package data

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

// CreateVerification 创建验证码挑战，并拒绝六十秒内向同一目标和场景重复发送。
func (r *AuthRepository) CreateVerification(ctx context.Context, record bizauth.VerificationRecord, resendWait time.Duration) (uint64, error) {
	var id uint64
	err := r.q.Transaction(func(tx *query.Query) error {
		if record.UserID != 0 {
			u := tx.User
			if _, err := u.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
				Select(u.ID).Where(u.ID.Eq(record.UserID)).Take(); err != nil {
				return err
			}
		}
		v := tx.VerificationCode
		count, err := v.WithContext(ctx).Where(
			v.Target.Eq(record.Target),
			v.Scene.Eq(record.Scene),
			v.CreatedAt.Gt(time.Now().UTC().Add(-resendWait)),
		).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return bizauth.ErrVerificationRateLimited
		}
		row := &model.VerificationCode{
			UserID: record.UserID, Target: record.Target, Scene: record.Scene, Channel: record.Channel,
			CodeHash: record.CodeHash, ExpiresAt: record.ExpiresAt, ContextData: datatypes.JSON(record.ContextData),
		}
		if err := v.WithContext(ctx).Create(row); err != nil {
			return err
		}
		id = row.ID
		return nil
	})
	return id, err
}

// ConsumeVerification 在行锁内校验并消费验证码，最多允许五次失败。
func (r *AuthRepository) ConsumeVerification(ctx context.Context, challengeID uint64, scene, codeHash string, now time.Time, maxAttempts uint32) (bizauth.VerificationRecord, error) {
	var result bizauth.VerificationRecord
	invalidCode := false
	err := r.q.Transaction(func(tx *query.Query) error {
		v := tx.VerificationCode
		row, err := v.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(v.ID.Eq(challengeID), v.Scene.Eq(scene)).Take()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return bizauth.ErrVerificationInvalid
			}
			return err
		}
		if row.ConsumedAt != nil || !row.ExpiresAt.After(now) || row.AttemptCount >= maxAttempts {
			return bizauth.ErrVerificationInvalid
		}
		if subtle.ConstantTimeCompare([]byte(row.CodeHash), []byte(codeHash)) != 1 {
			row.AttemptCount++
			assignments := []field.AssignExpr{v.AttemptCount.Value(row.AttemptCount)}
			if row.AttemptCount >= maxAttempts {
				assignments = append(assignments, v.ConsumedAt.Value(now))
			}
			if _, err := v.WithContext(ctx).Where(v.ID.Eq(row.ID)).UpdateSimple(assignments...); err != nil {
				return err
			}
			invalidCode = true
			return nil
		}
		if _, err := v.WithContext(ctx).Where(v.ID.Eq(row.ID)).Update(v.ConsumedAt, now); err != nil {
			return err
		}
		result = bizauth.VerificationRecord{
			ID: row.ID, UserID: row.UserID, Target: row.Target, Scene: row.Scene, Channel: row.Channel,
			CodeHash: row.CodeHash, AttemptCount: row.AttemptCount, ExpiresAt: row.ExpiresAt, ContextData: json.RawMessage(row.ContextData),
		}
		return nil
	})
	if err == nil && invalidCode {
		return bizauth.VerificationRecord{}, bizauth.ErrVerificationInvalid
	}
	return result, err
}

// UpdatePassword 更新 Argon2id 密码并撤销用户所有未过期会话。
func (r *AuthRepository) UpdatePassword(ctx context.Context, userID uint64, passwordHash string, changedAt time.Time) error {
	return r.q.Transaction(func(tx *query.Query) error {
		u := tx.User
		result, err := u.WithContext(ctx).
			Where(u.ID.Eq(userID), u.Status.Eq(1), u.DeletedAt.IsNull()).
			UpdateSimple(
				u.PasswordHash.Value(passwordHash),
				u.PasswordChangedAt.Value(changedAt),
				u.FailedLoginCount.Value(0),
				u.LockedUntil.Null(),
			)
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return bizauth.ErrAccountDisabled
		}
		s := tx.AuthSession
		_, err = s.WithContext(ctx).Where(s.UserID.Eq(userID), s.RevokedAt.IsNull()).Update(s.RevokedAt, changedAt)
		return err
	})
}

var _ bizauth.VerificationRepository = (*AuthRepository)(nil)

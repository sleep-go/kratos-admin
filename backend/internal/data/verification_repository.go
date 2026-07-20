package data

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
)

// CreateVerification 创建验证码挑战，并拒绝六十秒内向同一目标和场景重复发送。
func (r *AuthRepository) CreateVerification(ctx context.Context, record bizauth.VerificationRecord, resendWait time.Duration) (uint64, error) {
	var id uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.UserID != 0 {
			var lockedUserID uint64
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("users").Select("id").Where("id = ?", record.UserID).Scan(&lockedUserID).Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Table("verification_codes").Where(
			"target = ? AND scene = ? AND created_at > ?", record.Target, record.Scene, time.Now().UTC().Add(-resendWait),
		).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return bizauth.ErrVerificationRateLimited
		}
		row := &model.VerificationCode{
			UserID: record.UserID, Target: record.Target, Scene: record.Scene, Channel: record.Channel,
			CodeHash: record.CodeHash, ExpiresAt: record.ExpiresAt, ContextData: datatypes.JSON(record.ContextData),
		}
		if err := tx.Create(row).Error; err != nil {
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
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.VerificationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND scene = ?", challengeID, scene).Take(&row).Error; err != nil {
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
			updates := map[string]any{"attempt_count": row.AttemptCount}
			if row.AttemptCount >= maxAttempts {
				updates["consumed_at"] = now
			}
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return err
			}
			invalidCode = true
			return nil
		}
		if err := tx.Model(&row).Update("consumed_at", now).Error; err != nil {
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("users").Where("id = ? AND status = 1 AND deleted_at IS NULL", userID).Updates(map[string]any{
			"password_hash": passwordHash, "password_changed_at": changedAt,
			"failed_login_count": 0, "locked_until": nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return bizauth.ErrAccountDisabled
		}
		return tx.Table("auth_sessions").Where("user_id = ? AND revoked_at IS NULL", userID).Update("revoked_at", changedAt).Error
	})
}

var _ bizauth.VerificationRepository = (*AuthRepository)(nil)

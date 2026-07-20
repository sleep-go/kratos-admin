package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
)

const logCleanupBatchSize = 1000

// LogRetention 描述三类日志的保留天数。
type LogRetention struct {
	AuditDays uint32
	LoginDays uint32
	APIDays   uint32
}

// LogCleanupResult 描述单批日志清理条数。
type LogCleanupResult struct {
	Audit uint64
	Login uint64
	API   uint64
}

// LogMaintenanceRepository 按平台配置分批清理到期日志。
type LogMaintenanceRepository struct{ db *gorm.DB }

// NewLogMaintenanceRepository 创建日志保留策略仓储。
func NewLogMaintenanceRepository(data *Data) *LogMaintenanceRepository {
	return &LogMaintenanceRepository{db: data.DB}
}

// Retention 返回代码安全默认值与平台覆盖值合并后的保留策略。
func (r *LogMaintenanceRepository) Retention(ctx context.Context) (LogRetention, error) {
	retention := LogRetention{AuditDays: 365, LoginDays: 180, APIDays: 30}
	var rows []struct {
		SettingKey   string
		SettingValue []byte
	}
	if err := r.db.WithContext(ctx).Table("system_settings").
		Select("setting_key, setting_value").Where("tenant_id = 0 AND category = ?", "logs").Find(&rows).Error; err != nil {
		return LogRetention{}, err
	}
	for _, row := range rows {
		days, ok := retentionDays(row.SettingValue)
		if !ok {
			continue
		}
		switch row.SettingKey {
		case "audit_retention_days":
			retention.AuditDays = days
		case "login_retention_days":
			retention.LoginDays = days
		case "api_retention_days":
			retention.APIDays = days
		}
	}
	return retention, nil
}

// Cleanup 执行单批清理，避免长事务持续锁表。
func (r *LogMaintenanceRepository) Cleanup(ctx context.Context, now time.Time) (LogCleanupResult, error) {
	retention, err := r.Retention(ctx)
	if err != nil {
		return LogCleanupResult{}, err
	}
	result := LogCleanupResult{}
	for _, target := range []struct {
		table string
		days  uint32
		set   func(uint64)
	}{
		{table: "audit_logs", days: retention.AuditDays, set: func(value uint64) { result.Audit = value }},
		{table: "login_logs", days: retention.LoginDays, set: func(value uint64) { result.Login = value }},
		{table: "api_access_logs", days: retention.APIDays, set: func(value uint64) { result.API = value }},
	} {
		cutoff := now.UTC().AddDate(0, 0, -int(target.days))
		query := fmt.Sprintf("DELETE FROM %s WHERE created_at < ? ORDER BY created_at ASC LIMIT %d", target.table, logCleanupBatchSize)
		execution := r.db.WithContext(ctx).Exec(query, cutoff)
		if execution.Error != nil {
			return result, execution.Error
		}
		target.set(uint64(execution.RowsAffected))
	}
	return result, nil
}

func retentionDays(raw []byte) (uint32, bool) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false
	}
	var days uint64
	switch typed := value.(type) {
	case float64:
		if typed < 1 || typed > 3650 {
			return 0, false
		}
		days = uint64(typed)
	case string:
		parsed, err := strconv.ParseUint(typed, 10, 32)
		if err != nil || parsed < 1 || parsed > 3650 {
			return 0, false
		}
		days = parsed
	default:
		return 0, false
	}
	return uint32(days), true
}

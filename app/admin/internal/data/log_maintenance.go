package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gen"
	"gorm.io/gorm"

	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

const logCleanupBatchSize = 1000

const logMaintenanceLockName = "kratos_admin_log_maintenance"

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
type LogMaintenanceRepository struct {
	db *gorm.DB
	q  *query.Query
}

// NewLogMaintenanceRepository 创建日志保留策略仓储。
func NewLogMaintenanceRepository(data *Data) *LogMaintenanceRepository {
	return &LogMaintenanceRepository{db: data.DB, q: data.Query}
}

func (r *LogMaintenanceRepository) gen() *query.Query {
	if r.q != nil {
		return r.q
	}
	return query.Use(r.db)
}

// Retention 返回代码安全默认值与平台覆盖值合并后的保留策略。
func (r *LogMaintenanceRepository) Retention(ctx context.Context) (LogRetention, error) {
	retention := LogRetention{AuditDays: 365, LoginDays: 180, APIDays: 30}
	s := r.gen().SystemSetting
	rows, err := s.WithContext(ctx).Select(s.SettingKey, s.SettingValue).
		Where(s.TenantID.Eq(0), s.Category.Eq("logs")).Find()
	if err != nil {
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
	sqlDB, err := r.db.DB()
	if err != nil {
		return LogCleanupResult{}, fmt.Errorf("获取日志维护数据库连接池失败: %w", err)
	}
	connection, err := sqlDB.Conn(ctx)
	if err != nil {
		return LogCleanupResult{}, fmt.Errorf("获取日志维护专用连接失败: %w", err)
	}
	defer connection.Close()
	return runLockedMaintenance(ctx, sqlLockConnection{connection: connection}, func(ctx context.Context) (LogCleanupResult, error) {
		return r.cleanupUnlocked(ctx, now)
	})
}

func (r *LogMaintenanceRepository) cleanupUnlocked(ctx context.Context, now time.Time) (LogCleanupResult, error) {
	retention, err := r.Retention(ctx)
	if err != nil {
		return LogCleanupResult{}, err
	}
	result := LogCleanupResult{}
	for _, target := range []struct {
		days   uint32
		delete func(time.Time) (gen.ResultInfo, error)
		set    func(uint64)
	}{
		{days: retention.AuditDays, delete: func(cutoff time.Time) (gen.ResultInfo, error) {
			t := r.gen().AuditLog
			return t.WithContext(ctx).Where(t.CreatedAt.Lt(cutoff)).Order(t.CreatedAt.Asc()).Limit(logCleanupBatchSize).Delete()
		}, set: func(value uint64) { result.Audit = value }},
		{days: retention.LoginDays, delete: func(cutoff time.Time) (gen.ResultInfo, error) {
			t := r.gen().LoginLog
			return t.WithContext(ctx).Where(t.CreatedAt.Lt(cutoff)).Order(t.CreatedAt.Asc()).Limit(logCleanupBatchSize).Delete()
		}, set: func(value uint64) { result.Login = value }},
		{days: retention.APIDays, delete: func(cutoff time.Time) (gen.ResultInfo, error) {
			t := r.gen().APIAccessLog
			return t.WithContext(ctx).Where(t.CreatedAt.Lt(cutoff)).Order(t.CreatedAt.Asc()).Limit(logCleanupBatchSize).Delete()
		}, set: func(value uint64) { result.API = value }},
	} {
		cutoff := now.UTC().AddDate(0, 0, -int(target.days))
		execution, err := target.delete(cutoff)
		if err != nil {
			return result, err
		}
		target.set(uint64(execution.RowsAffected))
	}
	return result, nil
}

func runLockedMaintenance(ctx context.Context, connection lockConnection, cleanup func(context.Context) (LogCleanupResult, error)) (LogCleanupResult, error) {
	result := LogCleanupResult{}
	acquired, err := runWithNamedLock(ctx, connection, logMaintenanceLockName, 0, func(ctx context.Context) error {
		var cleanupErr error
		result, cleanupErr = cleanup(ctx)
		return cleanupErr
	})
	if err != nil {
		return LogCleanupResult{}, fmt.Errorf("执行日志维护失败: %w", err)
	}
	if !acquired {
		return LogCleanupResult{}, nil
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

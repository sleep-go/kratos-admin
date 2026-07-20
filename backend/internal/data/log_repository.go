package data

import (
	"context"
	"time"

	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

// RecordAccess 持久化不含请求正文的 API 访问与异常日志。
func (r *AuthRepository) RecordAccess(ctx context.Context, record service.AccessLogRecord) error {
	return r.db.WithContext(ctx).Create(&model.APIAccessLog{
		TenantID: record.TenantID, UserID: record.UserID, RequestID: record.RequestID,
		Method: record.Method, Route: record.Route, StatusCode: uint16(record.StatusCode), DurationMS: record.DurationMS,
		IP: record.IP, UserAgent: record.UserAgent, ErrorReason: record.ErrorReason, CreatedAt: time.Now().UTC(),
	}).Error
}

var _ service.AccessLogRecorder = (*AuthRepository)(nil)

// RecordLogin 持久化已脱敏的登录成功、失败、锁定与 MFA 事件。
func (r *AuthRepository) RecordLogin(ctx context.Context, record service.LoginLogRecord) error {
	return r.db.WithContext(ctx).Create(&model.LoginLog{
		TenantID: record.TenantID, UserID: record.UserID, Identifier: record.Identifier,
		Result: record.Result, Reason: record.Reason, IP: record.IP, UserAgent: record.UserAgent,
		RequestID: record.RequestID, CreatedAt: time.Now().UTC(),
	}).Error
}

var _ service.LoginLogRecorder = (*AuthRepository)(nil)

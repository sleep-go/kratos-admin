package audit

import "context"

// AccessLogRecord 描述不会包含请求正文和敏感字段的 API 访问日志。
type AccessLogRecord struct {
	TenantID    uint64
	Realm       string
	UserID      uint64
	RequestID   string
	Method      string
	Route       string
	StatusCode  int
	DurationMS  uint32
	IP          string
	UserAgent   string
	ErrorReason string
}

// AccessLogRecorder 定义 API 访问日志持久化能力。
type AccessLogRecorder interface {
	RecordAccess(context.Context, AccessLogRecord) error
}

// LoginLogRecord 描述已脱敏的登录安全事件。
type LoginLogRecord struct {
	TenantID   uint64
	Realm      string
	UserID     uint64
	Identifier string
	Result     uint8
	Reason     string
	IP         string
	UserAgent  string
	RequestID  string
}

// LoginLogRecorder 定义登录安全日志持久化能力。
type LoginLogRecorder interface {
	RecordLogin(context.Context, LoginLogRecord) error
}

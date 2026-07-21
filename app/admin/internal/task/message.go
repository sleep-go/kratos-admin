// Package task 管理 Admin 进程内的可靠异步任务发布与消费。
package task

import (
	"encoding/json"
	"fmt"
)

const (
	// ExchangeTasks 是业务异步任务的持久化交换机。
	ExchangeTasks = "kratos.admin.tasks.v1"
	// ExchangeDead 是非法任务进入死信队列时使用的交换机。
	ExchangeDead = "kratos.admin.tasks.dlx.v1"
	// QueueAudit 是审计 Outbox 消费队列。
	QueueAudit = "kratos.admin.audit.v1"
	// QueueLogExport 是日志导出消费队列。
	QueueLogExport = "kratos.admin.log_export.v1"
	// QueueFileCleanup 是文件对象清理消费队列。
	QueueFileCleanup = "kratos.admin.file_cleanup.v1"
	// QueueDead 是无法处理消息的统一死信队列。
	QueueDead = "kratos.admin.dead.v1"
	// RoutingAudit 是审计任务路由键。
	RoutingAudit = "audit.publish.v1"
	// RoutingLogExport 是日志导出任务路由键。
	RoutingLogExport = "log.export.v1"
	// RoutingFileCleanup 是文件清理任务路由键。
	RoutingFileCleanup = "file.cleanup.v1"
)

// Message 描述与 RabbitMQ 客户端解耦的稳定任务消息。
type Message struct {
	ID   string
	Type string
	Body []byte
}

type auditPayload struct {
	Version uint16 `json:"version"`
	EventID string `json:"event_id"`
}

type logExportPayload struct {
	Version  uint16 `json:"version"`
	ExportID string `json:"export_id"`
}

type fileCleanupPayload struct {
	Version      uint16 `json:"version"`
	TenantID     uint64 `json:"tenant_id"`
	FileID       string `json:"file_id"`
	ProviderName string `json:"provider_name"`
	ObjectKey    string `json:"object_key"`
}

// NewAuditMessage 创建版本化审计 Outbox 任务消息。
func NewAuditMessage(eventID string) (Message, error) {
	if eventID == "" {
		return Message{}, fmt.Errorf("审计任务缺少事件ID")
	}
	return newMessage("audit:"+eventID, RoutingAudit, auditPayload{Version: 1, EventID: eventID})
}

// NewLogExportMessage 创建版本化日志导出任务消息。
func NewLogExportMessage(exportID string) (Message, error) {
	if exportID == "" {
		return Message{}, fmt.Errorf("日志导出任务缺少导出ID")
	}
	return newMessage("log-export:"+exportID, RoutingLogExport, logExportPayload{Version: 1, ExportID: exportID})
}

// NewFileCleanupMessage 创建版本化文件对象清理任务消息。
func NewFileCleanupMessage(tenantID uint64, fileID, providerName, objectKey string) (Message, error) {
	if tenantID == 0 || fileID == "" || providerName == "" || objectKey == "" {
		return Message{}, fmt.Errorf("文件清理任务缺少必要字段")
	}
	return newMessage("file-cleanup:"+fileID, RoutingFileCleanup, fileCleanupPayload{
		Version: 1, TenantID: tenantID, FileID: fileID, ProviderName: providerName, ObjectKey: objectKey,
	})
}

func newMessage(id, messageType string, payload any) (Message, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("编码异步任务载荷失败: %w", err)
	}
	return Message{ID: id, Type: messageType, Body: body}, nil
}

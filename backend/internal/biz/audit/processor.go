// Package audit 负责将事务 Outbox 事件幂等转换为操作审计。
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrEventNotFound 表示待处理 Outbox 事件不存在。
var ErrEventNotFound = errors.New("审计Outbox事件不存在")

// Event 描述待消费的事务 Outbox 事件。
type Event struct {
	ID       string
	TenantID uint64
	Payload  []byte
}

// Entry 描述要持久化的操作审计内容。
type Entry struct {
	EventID      string
	TenantID     uint64
	UserID       uint64         `json:"user_id"`
	MemberID     uint64         `json:"member_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Summary      string         `json:"summary"`
	Before       map[string]any `json:"before"`
	After        map[string]any `json:"after"`
	IP           string         `json:"ip"`
	UserAgent    string         `json:"user_agent"`
	RequestID    string         `json:"request_id"`
}

// Repository 定义 Outbox 读取和审计幂等发布能力。
type Repository interface {
	Get(ctx context.Context, eventID string) (Event, error)
	Publish(ctx context.Context, event Event, entry Entry) (bool, error)
}

// Processor 将 Outbox 载荷转换为稳定审计记录。
type Processor struct{ repository Repository }

// NewProcessor 创建审计事件处理器。
func NewProcessor(repository Repository) *Processor { return &Processor{repository: repository} }

// Process 幂等处理指定 Outbox 事件。
func (p *Processor) Process(ctx context.Context, eventID string) error {
	event, err := p.repository.Get(ctx, eventID)
	if err != nil {
		return err
	}
	entry := Entry{EventID: event.ID, TenantID: event.TenantID}
	if err := json.Unmarshal(event.Payload, &entry); err != nil {
		return fmt.Errorf("解析审计事件失败: %w", err)
	}
	if entry.Summary == "" {
		entry.Summary = fmt.Sprintf("%s %s %s", entry.Action, entry.ResourceType, entry.ResourceID)
	}
	_, err = p.repository.Publish(ctx, event, entry)
	return err
}

package message

import (
	"context"
	"sync"
)

// LocalSender 是不输出敏感验证码的本地内存投递模拟器。
type LocalSender struct {
	channel string
	mu      sync.RWMutex
	items   map[string]CodeMessage
}

// NewLocalSender 创建指定渠道的本地投递模拟器。
func NewLocalSender(channel string) *LocalSender {
	return &LocalSender{channel: channel, items: map[string]CodeMessage{}}
}

// Channel 返回模拟器负责的渠道。
func (s *LocalSender) Channel() string { return s.channel }

// SendCode 将验证码保存在进程内存中，绝不写入日志。
func (s *LocalSender) SendCode(_ context.Context, message CodeMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[message.Target+":"+message.Scene] = message
	return nil
}

// LastCode 仅供本地验收和测试读取最近一次投递内容。
func (s *LocalSender) LastCode(target, scene string) (CodeMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	message, ok := s.items[target+":"+scene]
	return message, ok
}

var _ Sender = (*LocalSender)(nil)

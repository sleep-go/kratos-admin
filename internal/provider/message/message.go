// Package message 提供邮件与短信验证码的统一投递接口。
package message

import "context"

// CodeMessage 描述验证码投递内容。
type CodeMessage struct {
	Target  string
	Scene   string
	Code    string
	Minutes int
}

// Sender 定义邮件或短信验证码投递能力。
type Sender interface {
	Channel() string
	SendCode(ctx context.Context, message CodeMessage) error
}

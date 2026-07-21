// Package message 提供邮件与短信验证码的统一投递接口。
package message

import bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"

// CodeMessage 描述验证码投递内容。
type CodeMessage = bizauth.CodeMessage

// Sender 定义邮件或短信验证码投递能力。
type Sender = bizauth.MessageSender

package message

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig 描述 SMTP 邮件 Provider 配置。
type SMTPConfig struct {
	Address  string
	Host     string
	Username string
	Password string
	From     string
	UseTLS   bool
}

// SMTPSender 使用 SMTP 投递邮件验证码。
type SMTPSender struct{ config SMTPConfig }

// NewSMTPSender 创建 SMTP 邮件 Provider。
func NewSMTPSender(config SMTPConfig) (*SMTPSender, error) {
	if config.Address == "" || config.Host == "" || config.From == "" {
		return nil, errors.New("SMTP address、host 和 from 不能为空")
	}
	return &SMTPSender{config: config}, nil
}

// Channel 返回邮件渠道标识。
func (s *SMTPSender) Channel() string { return "email" }

// SendCode 通过 SMTP 投递验证码，正文不进入应用日志。
func (s *SMTPSender) SendCode(ctx context.Context, message CodeMessage) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}
	dialer := net.Dialer{Deadline: deadline}
	connection, err := dialer.DialContext(ctx, "tcp", s.config.Address)
	if err != nil {
		return err
	}
	defer connection.Close()
	client, err := smtp.NewClient(connection, s.config.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if s.config.UseTLS {
		if err := client.StartTLS(&tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(s.config.From); err != nil {
		return err
	}
	if err := client.Rcpt(message.Target); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	subject := "Kratos Admin 安全验证码"
	body := fmt.Sprintf("你的验证码为 %s，%d 分钟内有效。请勿向任何人泄露。", message.Code, message.Minutes)
	payload := strings.Join([]string{
		"From: " + s.config.From, "To: " + message.Target, "Subject: " + subject,
		"MIME-Version: 1.0", "Content-Type: text/plain; charset=UTF-8", "", body,
	}, "\r\n")
	if _, err := writer.Write([]byte(payload)); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

var _ Sender = (*SMTPSender)(nil)

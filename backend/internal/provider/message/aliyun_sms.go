package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysms "github.com/alibabacloud-go/dysmsapi-20170525/v4/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

// AliyunSMSConfig 描述阿里云短信 Provider 配置。
type AliyunSMSConfig struct {
	Region          string
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
}

type aliyunSMSClient interface {
	SendSmsWithOptions(*dysms.SendSmsRequest, *util.RuntimeOptions) (*dysms.SendSmsResponse, error)
	GetSmsSignWithOptions(*dysms.GetSmsSignRequest, *util.RuntimeOptions) (*dysms.GetSmsSignResponse, error)
}

// AliyunSMSSender 使用阿里云短信 OpenAPI 投递验证码。
type AliyunSMSSender struct {
	config AliyunSMSConfig
	client aliyunSMSClient
}

// NewAliyunSMSSender 创建阿里云短信 Provider。
func NewAliyunSMSSender(config AliyunSMSConfig) (*AliyunSMSSender, error) {
	if config.Region == "" || config.AccessKeyID == "" || config.AccessKeySecret == "" || config.SignName == "" || config.TemplateCode == "" {
		return nil, errors.New("阿里云短信 region、访问密钥、签名和模板不能为空")
	}
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "dysmsapi.aliyuncs.com"
	}
	client, err := dysms.NewClient(&openapi.Config{
		AccessKeyId: tea.String(config.AccessKeyID), AccessKeySecret: tea.String(config.AccessKeySecret),
		RegionId: tea.String(config.Region), Endpoint: tea.String(endpoint),
	})
	if err != nil {
		return nil, fmt.Errorf("初始化阿里云短信客户端失败: %w", err)
	}
	return newAliyunSMSSender(config, client), nil
}

func newAliyunSMSSender(config AliyunSMSConfig, client aliyunSMSClient) *AliyunSMSSender {
	return &AliyunSMSSender{config: config, client: client}
}

// Channel 返回短信渠道标识。
func (s *AliyunSMSSender) Channel() string { return "sms" }

// SendCode 使用已审核签名和模板发送短信验证码。
func (s *AliyunSMSSender) SendCode(ctx context.Context, message CodeMessage) error {
	params, _ := json.Marshal(map[string]string{"code": message.Code})
	response, err := s.client.SendSmsWithOptions(&dysms.SendSmsRequest{
		PhoneNumbers: tea.String(message.Target), SignName: tea.String(s.config.SignName),
		TemplateCode: tea.String(s.config.TemplateCode), TemplateParam: tea.String(string(params)),
	}, runtimeOptions(ctx))
	if err != nil {
		return err
	}
	if response == nil || response.Body == nil || tea.StringValue(response.Body.Code) != "OK" {
		return fmt.Errorf("阿里云短信发送失败: %s", responseMessage(response))
	}
	return nil
}

// TestConnection 查询已配置短信签名，验证凭证、网络和签名权限但不产生短信费用。
func (s *AliyunSMSSender) TestConnection(ctx context.Context) error {
	response, err := s.client.GetSmsSignWithOptions(
		&dysms.GetSmsSignRequest{SignName: tea.String(s.config.SignName)}, runtimeOptions(ctx),
	)
	if err != nil {
		return err
	}
	if response == nil || response.Body == nil || tea.StringValue(response.Body.Code) != "OK" {
		message := "响应无效"
		if response != nil && response.Body != nil {
			message = tea.StringValue(response.Body.Message)
		}
		return fmt.Errorf("阿里云短信连接测试失败: %s", message)
	}
	return nil
}

func runtimeOptions(ctx context.Context) *util.RuntimeOptions {
	timeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	if timeout <= 0 {
		timeout = time.Millisecond
	}
	milliseconds := int(timeout.Milliseconds())
	return &util.RuntimeOptions{ConnectTimeout: tea.Int(milliseconds), ReadTimeout: tea.Int(milliseconds), Autoretry: tea.Bool(false)}
}

func responseMessage(response *dysms.SendSmsResponse) string {
	if response == nil || response.Body == nil {
		return "响应无效"
	}
	return tea.StringValue(response.Body.Message)
}

var _ Sender = (*AliyunSMSSender)(nil)

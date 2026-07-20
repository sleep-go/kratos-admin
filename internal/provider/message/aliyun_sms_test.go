package message

import (
	"context"
	"testing"

	dysms "github.com/alibabacloud-go/dysmsapi-20170525/v4/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

type fakeAliyunSMSClient struct {
	sendRequest *dysms.SendSmsRequest
	signRequest *dysms.GetSmsSignRequest
}

func (c *fakeAliyunSMSClient) SendSmsWithOptions(request *dysms.SendSmsRequest, _ *util.RuntimeOptions) (*dysms.SendSmsResponse, error) {
	c.sendRequest = request
	return &dysms.SendSmsResponse{Body: &dysms.SendSmsResponseBody{Code: tea.String("OK"), Message: tea.String("OK")}}, nil
}

func (c *fakeAliyunSMSClient) GetSmsSignWithOptions(request *dysms.GetSmsSignRequest, _ *util.RuntimeOptions) (*dysms.GetSmsSignResponse, error) {
	c.signRequest = request
	return &dysms.GetSmsSignResponse{Body: &dysms.GetSmsSignResponseBody{Code: tea.String("OK"), Message: tea.String("OK")}}, nil
}

func TestAliyunSMSSenderSendsCodeWithConfiguredTemplate(t *testing.T) {
	client := &fakeAliyunSMSClient{}
	sender := newAliyunSMSSender(AliyunSMSConfig{SignName: "管理后台", TemplateCode: "SMS_1"}, client)
	if err := sender.SendCode(context.Background(), CodeMessage{Target: "13800138000", Code: "123456", Minutes: 5}); err != nil {
		t.Fatal(err)
	}
	if tea.StringValue(client.sendRequest.PhoneNumbers) != "13800138000" || tea.StringValue(client.sendRequest.SignName) != "管理后台" {
		t.Fatalf("request = %+v", client.sendRequest)
	}
	if tea.StringValue(client.sendRequest.TemplateParam) != `{"code":"123456"}` {
		t.Fatalf("template params = %s", tea.StringValue(client.sendRequest.TemplateParam))
	}
}

func TestAliyunSMSTestConnectionQueriesConfiguredSign(t *testing.T) {
	client := &fakeAliyunSMSClient{}
	sender := newAliyunSMSSender(AliyunSMSConfig{SignName: "管理后台", TemplateCode: "SMS_1"}, client)
	if err := sender.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tea.StringValue(client.signRequest.SignName) != "管理后台" {
		t.Fatalf("request = %+v", client.signRequest)
	}
}

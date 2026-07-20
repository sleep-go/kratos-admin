package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/transport"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
)

type testHeader map[string][]string

func (h testHeader) Get(key string) string {
	if len(h[key]) == 0 {
		return ""
	}
	return h[key][0]
}
func (h testHeader) Set(key, value string) { h[key] = []string{value} }
func (h testHeader) Add(key, value string) { h[key] = append(h[key], value) }
func (h testHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
func (h testHeader) Values(key string) []string { return h[key] }

type testTransport struct {
	operation string
	request   testHeader
	reply     testHeader
}

func (t *testTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (t *testTransport) Endpoint() string                { return "http://test" }
func (t *testTransport) Operation() string               { return t.operation }
func (t *testTransport) RequestHeader() transport.Header { return t.request }
func (t *testTransport) ReplyHeader() transport.Header   { return t.reply }

type fakeAccessValidator struct{ called bool }
type fakeAccessRecorder struct{ record AccessLogRecord }

func (r *fakeAccessRecorder) RecordAccess(_ context.Context, record AccessLogRecord) error {
	r.record = record
	return nil
}

func (v *fakeAccessValidator) ValidateAccess(_ context.Context, _ *bizauth.TokenClaims) error {
	v.called = true
	return nil
}

func TestAccessMiddlewareValidatesTokenAndStoresClaims(t *testing.T) {
	_, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	manager := bizauth.NewTokenManager(privateKey, time.Minute, time.Hour, nil)
	pair, _ := manager.Issue(bizauth.TokenSubject{UserID: 1, TenantID: 2, MemberID: 3, SessionID: "session"})
	validator := &fakeAccessValidator{}
	recorder := &fakeAccessRecorder{}
	service := NewAuthService(&fakeLoginHandler{}, false)
	service.ConfigureAccessSecurity(manager, validator)
	service.ConfigureAccessLog(recorder)
	ctx := transport.NewServerContext(context.Background(), &testTransport{
		operation: v1.OperationAuthServiceListSessions,
		request:   testHeader{"Authorization": {"Bearer " + pair.AccessToken}}, reply: testHeader{},
	})

	_, err := service.AccessMiddleware()(func(ctx context.Context, _ any) (any, error) {
		claims, ok := bizauth.ClaimsFromContext(ctx)
		if !ok || claims.TenantID != 2 {
			t.Fatalf("claims = %+v, ok %v", claims, ok)
		}
		return nil, nil
	})(ctx, nil)
	if err != nil || !validator.called {
		t.Fatalf("middleware err = %v, validator called = %v", err, validator.called)
	}
	if recorder.record.TenantID != 2 || recorder.record.UserID != 1 || recorder.record.RequestID == "" {
		t.Fatalf("access record = %+v", recorder.record)
	}
}

func TestAccessMiddlewareAllowsPublicLoginWithoutToken(t *testing.T) {
	service := NewAuthService(&fakeLoginHandler{}, false)
	ctx := transport.NewServerContext(context.Background(), &testTransport{
		operation: v1.OperationAuthServiceLogin, request: testHeader{}, reply: testHeader{},
	})
	called := false
	_, err := service.AccessMiddleware()(func(context.Context, any) (any, error) { called = true; return nil, nil })(ctx, nil)
	if err != nil || !called {
		t.Fatalf("public middleware err = %v, called = %v", err, called)
	}
}

func TestAccessMiddlewareRecordsUnauthorizedRequest(t *testing.T) {
	recorder := &fakeAccessRecorder{}
	service := NewAuthService(&fakeLoginHandler{}, false)
	service.ConfigureAccessLog(recorder)
	ctx := transport.NewServerContext(context.Background(), &testTransport{
		operation: v1.OperationAuthServiceListSessions, request: testHeader{}, reply: testHeader{},
	})

	_, err := service.AccessMiddleware()(func(context.Context, any) (any, error) {
		t.Fatal("protected handler must not be called")
		return nil, nil
	})(ctx, nil)
	if err == nil || recorder.record.StatusCode != 401 || recorder.record.ErrorReason != "AUTH_REQUIRED" {
		t.Fatalf("middleware err = %v, record = %+v", err, recorder.record)
	}
}

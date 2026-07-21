package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider"
	"github.com/sleep-go/kratos-admin/app/admin/internal/service"
)

func TestHTTPServerServesHealthCheck(t *testing.T) {
	healthService := service.NewHealthService("kratos-admin-api")
	httpServer := NewHTTPServer(
		conf.Config{Server: conf.Server{HTTPAddr: ":0"}},
		&service.Services{Health: healthService},
		&provider.AdminSet{},
	)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	httpServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	var reply v1.CheckResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if reply.Status != "ok" || reply.Service != "kratos-admin-api" {
		t.Fatalf("status = %q, service = %q, want ok health response", reply.Status, reply.Service)
	}
}

func TestEncodeErrorIncludesRequestID(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-Request-ID", "request-123")
	encodeError(recorder, httptest.NewRequest(http.MethodGet, "/", nil), kratoserrors.BadRequest("INVALID", "参数错误"))
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusBadRequest || body["request_id"] != "request-123" || body["reason"] != "INVALID" {
		t.Fatalf("response = status %d body %+v", recorder.Code, body)
	}
}

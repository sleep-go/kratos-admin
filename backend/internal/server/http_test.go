package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

func TestHTTPServerServesHealthCheck(t *testing.T) {
	healthService := service.NewHealthService("kratos-admin-api")
	httpServer := NewHTTPServer(conf.Server{HTTPAddr: ":0"}, healthService)
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

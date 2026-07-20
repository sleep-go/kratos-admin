// Package service 提供由 Kratos transport 暴露的应用服务实现。
package service

import (
	"context"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
)

// HealthService 实现部署探针使用的健康检查服务。
type HealthService struct {
	v1.UnimplementedHealthServiceServer
	serviceName string
}

// NewHealthService 创建健康检查服务。
func NewHealthService(serviceName string) *HealthService {
	return &HealthService{serviceName: serviceName}
}

// Check 返回进程可用状态与服务名称。
func (s *HealthService) Check(_ context.Context, _ *v1.CheckRequest) (*v1.CheckResponse, error) {
	return &v1.CheckResponse{Status: "ok", Service: s.serviceName}, nil
}

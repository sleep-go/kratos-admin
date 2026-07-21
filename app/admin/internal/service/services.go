package service

// Services 汇集 Admin Server 注册的全部 gRPC/HTTP 服务。
type Services struct {
	Health     *HealthService
	Auth       *AuthService
	Management *ManagementService
	File       *FileService
	Log        *LogService
}

// NewServices 汇集已经完成业务依赖注入的 Admin 服务。
func NewServices(health *HealthService, auth *AuthService, management *ManagementService, file *FileService, log *LogService) *Services {
	return &Services{Health: health, Auth: auth, Management: management, File: file, Log: log}
}

package service

// Services 汇集 Admin Server 注册的全部 gRPC/HTTP 服务。
type Services struct {
	Health       *HealthService
	Auth         *AuthService
	PlatformAuth *PlatformAuthService
	Management   *ManagementService
	File         *FileService
	Log          *LogService
}

// NewServices 汇集已经完成业务依赖注入的 Admin 服务。
func NewServices(
	health *HealthService,
	auth *AuthService,
	platformAuth *PlatformAuthService,
	management *ManagementService,
	file *FileService,
	log *LogService,
) *Services {
	return &Services{
		Health: health, Auth: auth, PlatformAuth: platformAuth,
		Management: management, File: file, Log: log,
	}
}

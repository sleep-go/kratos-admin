package service

import (
	"github.com/google/wire"
)

// ProviderSet 是 Admin 服务层的 Wire Provider 集合。
// 跨包的 wire.Bind（将 biz 层用例绑定到 service 层 Handler/Validator 接口）
// 因 Wire 要求与具体类型 provider 同 ProviderSet，统一放在 cmd/server/bindings.go。
var ProviderSet = wire.NewSet(
	NewServices,
	NewHealthService,
	NewAuthService,
	NewPlatformAuthService,
	NewManagementService,
	NewFileService,
	NewLogService,
)

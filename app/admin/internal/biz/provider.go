// Package biz 汇集业务逻辑层的 Wire Provider 集合。
package biz

import (
	"github.com/google/wire"

	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/audit"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/permission"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/providerconfig"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/setup"
)

// ProviderSet 聚合所有 biz 子包的 Wire Provider。
var ProviderSet = wire.NewSet(
	audit.ProviderSet,
	auth.ProviderSet,
	file.ProviderSet,
	logexport.ProviderSet,
	permission.ProviderSet,
	providerconfig.ProviderSet,
	setup.ProviderSet,
)

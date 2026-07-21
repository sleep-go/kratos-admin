package server

import (
	"github.com/google/wire"

	admintask "github.com/sleep-go/kratos-admin/app/admin/internal/server/task"
)

// ProviderSet 是 Admin Server 层的 Wire Provider 集合。
var ProviderSet = wire.NewSet(NewHTTPServer, NewGRPCServer, admintask.NewServer)

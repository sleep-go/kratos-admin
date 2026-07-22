package server

import (
	v1 "dps/api/kratos/admin/v1"
	"dps/internal/conf"
	"dps/internal/service"
	"dps/pkg/auth"
	"dps/pkg/validate"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, admin *service.AdminService) *http.Server {
	var opts = []http.ServerOption{
		http.Filter(
			auth.Middleware(),
		),
		http.Middleware(
			recovery.Recovery(),
			validate.Middleware(),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	v1.RegisterAdminServiceHTTPServer(srv, admin)
	return srv
}

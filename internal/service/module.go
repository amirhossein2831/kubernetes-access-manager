package service

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/service/monitor"
	"go.uber.org/fx"
)

var Module = fx.Module("service",
	fx.Provide(monitor.Init),
)

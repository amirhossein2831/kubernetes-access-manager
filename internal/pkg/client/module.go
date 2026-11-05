package client

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/client/http/kubernetes"
	"go.uber.org/fx"
)

var Module = fx.Module("client",
	fx.Provide(kubernetes.Init),
)

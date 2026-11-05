package service

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/logger"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/client/http/kubernetes"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/service/monitor"
	"go.uber.org/fx"
)

var Module = fx.Module("service",
	fx.Provide(
		func(client *kubernetes.Client) monitor.Fetcher { return client },
		func(writer *logger.Writer) monitor.Emitter { return writer },
		monitor.New,
	),
)

package main

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/jobs"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/logger"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/client"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/service"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport"
	"go.uber.org/fx"
	"time"
)

func main() {
	fx.New(
		config.Module,
		observability.Module,
		logger.Module,
		client.Module,
		service.Module,
		transport.Module,
		jobs.Module,
		fx.WithLogger(logger.FrameworkLogger),
		fx.StartTimeout(30*time.Second),
	).Run()
}

package main

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/jobs"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/logger"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/client"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/service"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport"
	"testing"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"go.uber.org/fx"
)

func options() fx.Option {
	return fx.Options(
		config.Module,
		observability.Module,
		logger.Module,
		client.Module,
		service.Module,
		transport.Module,
		jobs.Module,
		fx.WithLogger(logger.FrameworkLogger),
	)
}

func TestPackageInitializersProvideApplicationGraph(t *testing.T) {
	// Validate only: no constructors run, credentials are not read, and no
	// listeners, collection loops or cluster connections are started.
	if err := fx.ValidateApp(options()); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidConfigurationFailsFxConstruction(t *testing.T) {
	t.Chdir("../..")
	t.Setenv("APP_ENV", "local")
	t.Setenv("APP_POLL_INTERVAL", "0s")
	app := fx.New(options(), fx.NopLogger, fx.Invoke(func(*config.Config) {}))
	if app.Err() == nil {
		t.Fatal("invalid configuration did not fail Fx construction")
	}
}

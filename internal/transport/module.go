package transport

import (
	"net/http"

	httptransport "github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http/controller/health"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http/router"
	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
)

var Module = fx.Module("transport",
	fx.Provide(
		httptransport.NewServer,
		router.New,
		func(mux *chi.Mux) http.Handler { return mux },
		fx.Annotate(health.New,
			fx.As(new(router.Registrar)),
			fx.ResultTags(`group:"routes"`),
		),
	),
	fx.Invoke(func(*http.Server) {}),
)

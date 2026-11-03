package transport

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	httptransport "github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http/router"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
	"net/http"
)

type routeParams struct {
	fx.In
	Metrics    *observability.Metrics
	Registry   *prometheus.Registry
	Registrars []router.Registrar `group:"routes"`
}

var Module = fx.Module("transport", fx.Provide(func(p routeParams) *chi.Mux {
	return router.New(router.Params{Metrics: p.Metrics, Gatherer: p.Registry, Registrars: p.Registrars})
}, func(r *chi.Mux) http.Handler { return r }, httptransport.NewServer), fx.Invoke(func(*http.Server) {}))

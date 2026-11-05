package router

import (
	"net/http"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http/httpio"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
)

type Registrar interface {
	Routes(chi.Router)
}

type Params struct {
	fx.In
	Gatherer   prometheus.Gatherer
	Registrars []Registrar `group:"routes"`
}

func New(p Params) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)

	r.Handle("/metrics", promhttp.HandlerFor(p.Gatherer, promhttp.HandlerOpts{}))

	r.NotFound(httpio.Wrap(func(*http.Request) (*httpio.Response, error) {
		return httpio.JSON(404, map[string]string{"error": "not found", "code": "not_found"}), nil
	}))

	for _, registrar := range p.Registrars {
		registrar.Routes(r)
	}

	return r
}

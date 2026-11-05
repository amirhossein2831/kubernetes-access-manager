package router

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	servicehealth "github.com/amirhossein2831/kubernetes-access-manager/internal/observability/health"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http/controller/health"
	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAndMetricsContracts(t *testing.T) {
	servicehealth.Ready.Store(false)
	t.Cleanup(func() { servicehealth.Ready.Store(false) })
	var router *chi.Mux
	app := fx.New(
		fx.NopLogger,
		observability.Module,
		fx.Provide(
			New,
			fx.Annotate(health.New,
				fx.As(new(Registrar)),
				fx.ResultTags(`group:"routes"`),
			),
		),
		fx.Populate(&router),
	)
	if err := app.Err(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want int
	}{{
		"/livez",
		200,
	}, {
		"/readyz",
		503,
	}, {
		"/metrics",
		200,
	}, {
		"/missing",
		404,
	}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s=%d", tc.path, w.Code)
		}
	}
	servicehealth.Ready.Store(true)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatal("startup readiness not reflected")
	}
}

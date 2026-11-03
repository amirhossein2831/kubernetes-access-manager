package router

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAndMetricsContracts(t *testing.T) {
	r := prometheus.NewRegistry()
	m := observability.New(r)
	router := New(Params{Metrics: m, Gatherer: r})
	for _, tc := range []struct {
		path string
		want int
	}{{"/livez", 200}, {"/readyz", 503}, {"/metrics", 200}, {"/missing", 404}} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s=%d", tc.path, w.Code)
		}
	}
	m.SetReady(true)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatal("startup readiness not reflected")
	}
}

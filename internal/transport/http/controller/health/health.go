package health

import (
	"net/http"

	servicehealth "github.com/amirhossein2831/kubernetes-access-manager/internal/observability/health"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http/httpio"
	"github.com/go-chi/chi/v5"
)

type Controller struct{}

func New() *Controller {
	return &Controller{}
}

func (c *Controller) Routes(r chi.Router) {
	r.Get("/livez", httpio.Wrap(c.Live))
	r.Get("/readyz", httpio.Wrap(c.Ready))
}

func (c *Controller) Live(*http.Request) (*httpio.Response, error) {
	return httpio.JSON(200, map[string]string{"status": "ok"}), nil
}

func (c *Controller) Ready(*http.Request) (*httpio.Response, error) {
	if !servicehealth.Ready.Load() {
		return httpio.JSON(503, map[string]string{"status": "initializing"}), nil
	}

	return httpio.JSON(200, map[string]string{"status": "ready"}), nil
}

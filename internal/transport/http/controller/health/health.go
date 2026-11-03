package health

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/transport/http/httpio"
	"net/http"
)

type Controller struct{ metrics *observability.Metrics }

func New(m *observability.Metrics) *Controller { return &Controller{m} }
func (c *Controller) Live(*http.Request) (*httpio.Response, error) {
	return httpio.JSON(200, map[string]string{"status": "ok"}), nil
}
func (c *Controller) Ready(*http.Request) (*httpio.Response, error) {
	if !c.metrics.Ready() {
		return httpio.JSON(503, map[string]string{"status": "initializing"}), nil
	}
	return httpio.JSON(200, map[string]string{"status": "ready"}), nil
}

package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability/metric"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Init(c *config.Config) (*Client, error) {
	client, err := New(c)
	if err == nil {
		client.Observe = func(operation, result string, duration time.Duration) {
			metric.KubernetesRequestsTotal.WithLabelValues(operation, result).Inc()
			metric.KubernetesRequestDuration.WithLabelValues(operation).Observe(duration.Seconds())
		}
	}
	return client, err
}

type Client struct {
	cfg     *config.Config
	http    *http.Client
	base    string
	limiter *rate.Limiter
	Observe func(operation, result string, duration time.Duration)
}

func New(cfg *config.Config) (*Client, error) {
	raw, err := os.ReadFile(cfg.Kubernetes.Kubeconfig)
	if err != nil {
		return nil, errors.New("cannot read kubeconfig file")
	}

	if err := ValidateKubeconfig(raw); err != nil {
		return nil, err
	}

	rc, err := clientcmd.RESTConfigFromKubeConfig(raw)

	if err != nil {
		return nil, errors.New("cannot initialize Kubernetes configuration")
	}

	return NewWithREST(cfg, rc)
}

func NewWithREST(cfg *config.Config, rc *rest.Config) (*Client, error) {
	if rc.ExecProvider != nil || rc.Impersonate.UserName != "" || rc.Insecure {
		return nil, errors.New("unsafe Kubernetes REST configuration")
	}
	rc = rest.CopyConfig(rc)
	rc.Timeout = 0
	rc.UserAgent = "access-monitor/v1"
	h, err := rest.HTTPClientFor(rc)
	if err != nil {
		return nil, errors.New("cannot construct Kubernetes transport")
	}
	return &Client{
		cfg:     cfg,
		http:    h,
		base:    strings.TrimRight(rc.Host, "/"),
		limiter: rate.NewLimiter(rate.Limit(cfg.Kubernetes.QPS), cfg.Kubernetes.Burst),
	}, nil
}

func (c *Client) get(ctx context.Context, path, operation string, out any) error {
	var last error
	for attempt := 0; attempt < c.cfg.Kubernetes.Attempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}
		reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Kubernetes.RequestTimeout)
		req, err := c.newRequest(reqCtx, path)
		if err != nil {
			cancel()
			return err
		}
		started := time.Now()
		response, err := c.http.Do(req)
		status := 0
		delay := time.Duration(0)
		result := "transport_error"
		if err == nil {
			status = response.StatusCode
			delay = retryAfter(response.Header.Get("Retry-After"))
			if status >= 200 && status < 300 {
				err = decodeResponse(response.Body, out)
				response.Body.Close()
				cancel()
				result = "success"
				if err != nil {
					result = "decode_error"
				}
				if c.Observe != nil {
					c.Observe(operation, result, time.Since(started))
				}
				return err
			}
			io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			response.Body.Close()
			result = strconv.Itoa(status)
		}
		cancel()
		if c.Observe != nil {
			c.Observe(operation, result, time.Since(started))
		}
		last = fmt.Errorf("kubernetes %s failed (%s)", operation, result)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if status != 0 && status != 429 && status != 500 && status != 502 && status != 503 && status != 504 {
			return last
		}
		if attempt+1 == c.cfg.Kubernetes.Attempts {
			break
		}
		backoff := min(c.cfg.Kubernetes.BackoffMin*time.Duration(1<<attempt), c.cfg.Kubernetes.BackoffMax)
		backoff = max(delay, time.Duration(float64(backoff)*(0.5+rand.Float64()*0.5)))
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}

func (c *Client) list(ctx context.Context, path, operation string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	continuation := ""
	seen := map[string]bool{}
	for {
		var page listPage
		if err := c.get(ctx, listPath(path, c.cfg.Kubernetes.PageSize, continuation), operation, &page); err != nil {
			return nil, err
		}
		if page.Items == nil {
			return nil, errors.New("kubernetes list response missing items")
		}
		all = append(all, page.Items...)
		continuation = page.Metadata.Continue
		if continuation == "" {
			return all, nil
		}
		if seen[continuation] {
			return nil, errors.New("repeated Kubernetes continuation token")
		}
		seen[continuation] = true
	}
}

func (c *Client) Fetch(ctx context.Context) (domain.State, error) {
	state := domain.State{}
	for _, kind := range []string{"roles", "clusterroles", "rolebindings", "clusterrolebindings"} {
		items, err := c.list(ctx, "/apis/rbac.authorization.k8s.io/v1/"+kind, kind)
		if err != nil {
			return state, err
		}
		for _, raw := range items {
			if err := appendRBAC(&state, kind, raw); err != nil {
				return state, err
			}
		}
	}
	namespaces, err := c.list(ctx, "/api/v1/namespaces", "namespaces")
	if err != nil {
		return state, err
	}
	for _, raw := range namespaces {
		name, err := namespaceName(raw)
		if err != nil {
			return state, err
		}
		state.Namespaces = append(state.Namespaces, name)
	}
	var core metav1.APIVersions
	if err := c.get(ctx, "/api", "discovery", &core); err != nil {
		return state, err
	}
	var paths []string
	for _, version := range core.Versions {
		paths = append(paths, "/api/"+version)
	}
	var groups metav1.APIGroupList
	if err := c.get(ctx, "/apis", "discovery", &groups); err != nil {
		return state, err
	}
	for _, group := range groups.Groups {
		if group.PreferredVersion.GroupVersion != "" {
			paths = append(paths, "/apis/"+group.PreferredVersion.GroupVersion)
		}
	}
	if len(paths) == 0 {
		return state, errors.New("empty Kubernetes discovery")
	}
	seen := map[string]bool{}
	for _, path := range paths {
		var list metav1.APIResourceList
		if err := c.get(ctx, path, "discovery", &list); err != nil {
			return state, err
		}
		appendResources(&state, list, seen)
	}
	return state, nil
}

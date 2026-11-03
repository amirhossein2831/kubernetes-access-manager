package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"golang.org/x/time/rate"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	cfg     *config.Config
	http    *http.Client
	base    string
	limiter *rate.Limiter
	Observe func(operation, result string, duration time.Duration)
}

func ValidateKubeconfig(raw []byte) error {
	c, err := clientcmd.Load(raw)
	if err != nil {
		return errors.New("invalid kubeconfig syntax")
	}
	for _, a := range c.AuthInfos {
		if a.Exec != nil {
			return errors.New("kubeconfig exec authentication is not supported")
		}
		if a.Impersonate != "" || len(a.ImpersonateGroups) > 0 || len(a.ImpersonateUserExtra) > 0 || a.ImpersonateUID != "" {
			return errors.New("kubeconfig impersonation is not allowed")
		}
	}
	for _, cluster := range c.Clusters {
		if cluster.InsecureSkipTLSVerify {
			return errors.New("insecure Kubernetes TLS configuration is not allowed")
		}
	}
	return nil
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
	return &Client{cfg: cfg, http: h, base: strings.TrimRight(rc.Host, "/"), limiter: rate.NewLimiter(rate.Limit(cfg.Kubernetes.QPS), cfg.Kubernetes.Burst)}, nil
}

func (c *Client) get(ctx context.Context, path, operation string, out any) error {
	var last error
	for attempt := 0; attempt < c.cfg.Kubernetes.Attempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}
		reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Kubernetes.RequestTimeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.base+path, nil)
		if err != nil {
			cancel()
			return err
		}
		req.Header.Set("Accept", "application/json")
		started := time.Now()
		response, err := c.http.Do(req)
		status := 0
		delay := time.Duration(0)
		result := "transport_error"
		if err == nil {
			status = response.StatusCode
			if hint := response.Header.Get("Retry-After"); hint != "" {
				if seconds, e := strconv.Atoi(hint); e == nil && seconds >= 0 {
					delay = time.Duration(seconds) * time.Second
				} else if date, e := http.ParseTime(hint); e == nil {
					delay = time.Until(date)
				}
			}
			if status >= 200 && status < 300 {
				decoder := json.NewDecoder(io.LimitReader(response.Body, 32*1024*1024))
				err = decoder.Decode(out)
				if err == nil {
					var extra any
					if decoder.Decode(&extra) != io.EOF {
						err = errors.New("invalid trailing Kubernetes response")
					}
				}
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
		last = fmt.Errorf("Kubernetes %s failed (%s)", operation, result)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if status != 0 && status != 429 && status != 500 && status != 502 && status != 503 && status != 504 {
			return last
		}
		if attempt+1 == c.cfg.Kubernetes.Attempts {
			break
		}
		backoff := c.cfg.Kubernetes.BackoffMin * time.Duration(1<<attempt)
		if backoff > c.cfg.Kubernetes.BackoffMax {
			backoff = c.cfg.Kubernetes.BackoffMax
		}
		backoff = time.Duration(float64(backoff) * (0.5 + rand.Float64()*0.5))
		if delay > backoff {
			backoff = delay
		}
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

type listPage struct {
	Metadata metav1.ListMeta   `json:"metadata"`
	Items    []json.RawMessage `json:"items"`
}

func (c *Client) list(ctx context.Context, path, operation string) ([]json.RawMessage, error) {
	all := []json.RawMessage{}
	continuation := ""
	seen := map[string]bool{}
	for {
		query := url.Values{"limit": []string{strconv.FormatInt(c.cfg.Kubernetes.PageSize, 10)}}
		if continuation != "" {
			query.Set("continue", continuation)
		}
		var page listPage
		if err := c.get(ctx, path+"?"+query.Encode(), operation, &page); err != nil {
			return nil, err
		}
		if page.Items == nil {
			return nil, errors.New("Kubernetes list response missing items")
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
			switch kind {
			case "roles":
				var r rbac.Role
				if err := json.Unmarshal(raw, &r); err != nil {
					return state, err
				}
				state.Roles = append(state.Roles, convertRole("Role", r.ObjectMeta, r.Rules))
			case "clusterroles":
				var r rbac.ClusterRole
				if err := json.Unmarshal(raw, &r); err != nil {
					return state, err
				}
				state.Roles = append(state.Roles, convertRole("ClusterRole", r.ObjectMeta, r.Rules))
			case "rolebindings":
				var b rbac.RoleBinding
				if err := json.Unmarshal(raw, &b); err != nil {
					return state, err
				}
				state.Bindings = append(state.Bindings, convertBinding("RoleBinding", b.ObjectMeta, b.RoleRef, b.Subjects))
			case "clusterrolebindings":
				var b rbac.ClusterRoleBinding
				if err := json.Unmarshal(raw, &b); err != nil {
					return state, err
				}
				state.Bindings = append(state.Bindings, convertBinding("ClusterRoleBinding", b.ObjectMeta, b.RoleRef, b.Subjects))
			}
		}
	}
	namespaces, err := c.list(ctx, "/api/v1/namespaces", "namespaces")
	if err != nil {
		return state, err
	}
	for _, raw := range namespaces {
		var ns struct {
			Metadata metav1.ObjectMeta `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &ns); err != nil {
			return state, err
		}
		state.Namespaces = append(state.Namespaces, ns.Metadata.Name)
	}
	var core metav1.APIVersions
	if err := c.get(ctx, "/api", "discovery", &core); err != nil {
		return state, err
	}
	paths := []string{}
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
		group := ""
		if parts := strings.SplitN(list.GroupVersion, "/", 2); len(parts) == 2 {
			group = parts[0]
		}
		for _, resource := range list.APIResources {
			key := group + "/" + resource.Name
			if !seen[key] {
				state.Resources = append(state.Resources, domain.Resource{APIGroup: group, Name: resource.Name, Namespaced: resource.Namespaced})
				seen[key] = true
			}
		}
	}
	return state, nil
}
func convertRole(kind string, m metav1.ObjectMeta, rules []rbac.PolicyRule) domain.Role {
	r := domain.Role{Kind: kind, Name: m.Name, Namespace: m.Namespace, UID: string(m.UID)}
	for _, rule := range rules {
		r.Rules = append(r.Rules, domain.Rule{APIGroups: rule.APIGroups, Resources: rule.Resources, Verbs: rule.Verbs, ResourceNames: rule.ResourceNames, NonResourceURLs: rule.NonResourceURLs})
	}
	return r
}
func convertBinding(kind string, m metav1.ObjectMeta, ref rbac.RoleRef, subjects []rbac.Subject) domain.Binding {
	b := domain.Binding{Kind: kind, Name: m.Name, Namespace: m.Namespace, UID: string(m.UID), RoleKind: ref.Kind, RoleName: ref.Name}
	for _, s := range subjects {
		b.Subjects = append(b.Subjects, domain.Subject{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace})
	}
	return b
}

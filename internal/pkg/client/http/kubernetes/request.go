package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"k8s.io/client-go/tools/clientcmd"
)

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

func (c *Client) newRequest(ctx context.Context, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func listPath(path string, pageSize int64, continuation string) string {
	query := url.Values{
		"limit": []string{strconv.FormatInt(pageSize, 10)},
	}
	if continuation != "" {
		query.Set("continue", continuation)
	}
	return path + "?" + query.Encode()
}

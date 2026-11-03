package kubernetes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"k8s.io/client-go/rest"
)

func settings(t *testing.T) *config.Config {
	t.Helper()
	c, err := config.LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.Kubernetes.BackoffMin = time.Millisecond
	c.Kubernetes.BackoffMax = time.Millisecond
	return c
}

func TestFetchPaginationAndDiscovery(t *testing.T) {
	var pages atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apis/rbac.authorization.k8s.io/v1/roles":
			if r.URL.Query().Get("continue") == "next" {
				pages.Add(1)
				fmt.Fprint(w, `{"items":[{"metadata":{"name":"second","namespace":"dev"},"rules":[]}]}`)
			} else {
				fmt.Fprint(w, `{"metadata":{"continue":"next"},"items":[{"metadata":{"name":"first","namespace":"dev"},"rules":[{"apiGroups":[""],"resources":["pods"],"verbs":["get"]}]}]}`)
			}
		case "/apis/rbac.authorization.k8s.io/v1/rolebindings", "/apis/rbac.authorization.k8s.io/v1/clusterroles", "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings":
			fmt.Fprint(w, `{"items":[]}`)
		case "/api/v1/namespaces":
			fmt.Fprint(w, `{"items":[{"metadata":{"name":"dev"}}]}`)
		case "/api":
			fmt.Fprint(w, `{"versions":["v1"]}`)
		case "/apis":
			fmt.Fprint(w, `{"groups":[]}`)
		case "/api/v1":
			fmt.Fprint(w, `{"groupVersion":"v1","resources":[{"name":"pods","namespaced":true},{"name":"pods/exec","namespaced":true},{"name":"nodes","namespaced":false}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewWithREST(settings(t), &rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	state, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Roles) != 2 || pages.Load() != 1 || len(state.Resources) != 3 || len(state.Namespaces) != 1 {
		t.Fatalf("incomplete state: %+v", state)
	}
}

func TestBoundedRetriesAndPermanentFailures(t *testing.T) {
	for _, status := range []int{403, 401, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(status) }))
			defer s.Close()
			c, _ := NewWithREST(settings(t), &rest.Config{Host: s.URL})
			_, err := c.Fetch(context.Background())
			if err == nil {
				t.Fatal("failed request accepted")
			}
			want := int32(3)
			if status == 401 || status == 403 {
				want = 1
			}
			if calls.Load() != want {
				t.Fatalf("attempts=%d want=%d", calls.Load(), want)
			}
		})
	}
}

func TestCancellationAndUntrustedAuthentication(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	c, _ := NewWithREST(settings(t), &rest.Config{Host: s.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.Fetch(ctx); err == nil {
		t.Fatal("cancel ignored")
	}
	for _, raw := range []string{
		"apiVersion: v1\nkind: Config\nclusters: []\nusers:\n- name: test\n  user:\n    exec:\n      command: untrusted\ncontexts: []\n",
		"apiVersion: v1\nkind: Config\nusers:\n- name: test\n  user:\n    as: admin\n",
	} {
		if err := ValidateKubeconfig([]byte(raw)); err == nil {
			t.Fatal("unsafe kubeconfig accepted")
		}
	}
	if err := ValidateKubeconfig([]byte("apiVersion: v1\nkind: Config\nusers: []\n")); err != nil && strings.Contains(err.Error(), "exec") {
		t.Fatal(err)
	}
}

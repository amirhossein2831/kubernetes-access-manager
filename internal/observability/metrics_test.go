package observability

import (
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	"testing"
	"time"
)

func TestUnknownCountsAbsentAndPublicationStable(t *testing.T) {
	r := prometheus.NewRegistry()
	m := New(r)
	families, err := r.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == "k8s_namespace_sensitive_access_users_count" {
			t.Fatal("unknown access reported as zero")
		}
	}
	m.Publish(domain.Counts{Namespace: map[domain.NamespaceKey]int{{Namespace: "kube-system", Resource: "secrets", Verb: "get"}: 2}, Cluster: map[domain.PermissionKey]int{}}, time.Unix(100, 0))
	m.Collection("failure", time.Second)
	families, err = r.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range families {
		if f.GetName() == "k8s_namespace_sensitive_access_users_count" {
			if f.Metric[0].Gauge.GetValue() != 2 {
				t.Fatal("failure changed counts")
			}
			found = true
		}
		if f.GetName() == "access_monitor_last_successful_observation_timestamp_seconds" && f.Metric[0].Gauge.GetValue() != 100 {
			t.Fatal("advanced freshness")
		}
	}
	if !found {
		t.Fatal("missing published counts")
	}
}

package metric

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestAccessMetricLabelsAndReset(t *testing.T) {
	NamespaceSensitiveAccessUsers.Reset()
	ClusterSensitiveAccessUsers.Reset()
	t.Cleanup(func() {
		NamespaceSensitiveAccessUsers.Reset()
		ClusterSensitiveAccessUsers.Reset()
	})
	NamespaceSensitiveAccessUsers.WithLabelValues("default", "get", "secrets").Set(2)
	ClusterSensitiveAccessUsers.WithLabelValues("secrets", "get").Set(1)
	value := &dto.Metric{}
	if err := NamespaceSensitiveAccessUsers.WithLabelValues("default", "get", "secrets").Write(value); err != nil {
		t.Fatal(err)
	}
	if got := value.GetGauge().GetValue(); got != 2 {
		t.Fatalf("namespace count = %v", got)
	}
	if err := ClusterSensitiveAccessUsers.WithLabelValues("secrets", "get").Write(value); err != nil {
		t.Fatal(err)
	}
	if got := value.GetGauge().GetValue(); got != 1 {
		t.Fatalf("cluster count = %v", got)
	}
	NamespaceSensitiveAccessUsers.Reset()
	ClusterSensitiveAccessUsers.Reset()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "k8s_namespace_sensitive_access_users_count" || family.GetName() == "k8s_cluster_wide_sensitive_access_users_count" {
			t.Fatal("reset retained obsolete access series")
		}
	}
}

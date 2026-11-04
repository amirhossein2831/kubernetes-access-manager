package metric

import (
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func CollectionSucceeded(counts domain.Counts, observed, start time.Time) {
	NamespaceSensitiveAccessUsers.Reset()
	ClusterSensitiveAccessUsers.Reset()
	for key, count := range counts.Namespace {
		NamespaceSensitiveAccessUsers.WithLabelValues(key.Namespace, key.Verb, key.Resource).Set(float64(count))
	}
	for key, count := range counts.Cluster {
		ClusterSensitiveAccessUsers.WithLabelValues(key.Resource, key.Verb).Set(float64(count))
	}
	LastSuccessfulObservation.Set(float64(observed.Unix()))
	HasObservation.Set(1)
	CollectionsTotal.WithLabelValues("success").Inc()
	CollectionDuration.Observe(time.Since(start).Seconds())
}

func CollectionFailed(start time.Time) {
	CollectionsTotal.WithLabelValues("failure").Inc()
	CollectionDuration.Observe(time.Since(start).Seconds())
}

var NamespaceSensitiveAccessUsers = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "k8s_namespace_sensitive_access_users_count",
		Help: "Distinct monitored users with matching grants, including usable named grants.",
	},
	[]string{"namespace", "verb", "resource"},
)

var ClusterSensitiveAccessUsers = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "k8s_cluster_wide_sensitive_access_users_count",
		Help: "Distinct monitored users with all-namespace or cluster-scoped matching grants.",
	},
	[]string{"resource", "verb"},
)

var LastSuccessfulObservation = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "access_monitor_last_successful_observation_timestamp_seconds",
	Help: "Last complete resolved observation; unchanged on failure.",
})

var HasObservation = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "access_monitor_has_observation",
	Help: "Whether any complete observation has succeeded.",
})

var CollectionsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "access_monitor_collections_total",
		Help: "Collection outcomes.",
	},
	[]string{"result"},
)

var CollectionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name: "access_monitor_collection_duration_seconds",
	Help: "Collection duration.",
})

var KubernetesRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "access_monitor_kubernetes_requests_total",
		Help: "Kubernetes request attempts.",
	},
	[]string{"operation", "result"},
)

var KubernetesRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "access_monitor_kubernetes_request_duration_seconds",
		Help: "Kubernetes attempt duration.",
	},
	[]string{"operation"},
)

var OutputTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "access_monitor_output_total",
		Help: "Output attempts by sink and result.",
	},
	[]string{"sink", "result"},
)

var OutputDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "access_monitor_output_duration_seconds",
		Help: "Output write duration.",
	},
	[]string{"sink"},
)

var StageDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "access_monitor_stage_duration_seconds",
		Help: "Processing stage duration.",
	},
	[]string{"stage"},
)

var StageFailuresTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "access_monitor_stage_failures_total",
		Help: "Failed processing stages.",
	},
	[]string{"stage"},
)

var LoadedUsers = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "access_monitor_loaded_users",
	Help: "Validated monitored inventory size.",
})

var LastFullFileReport = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "access_monitor_last_full_file_report_timestamp_seconds",
	Help: "Last complete report accepted by file sink; not delivery acknowledgement.",
})

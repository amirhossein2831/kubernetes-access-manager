package observability

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Metrics struct {
	mu                            sync.RWMutex
	counts                        domain.Counts
	at                            time.Time
	ready                         atomic.Bool
	namespace, cluster, last, has *prometheus.Desc
	collections                   *prometheus.CounterVec
	duration                      prometheus.Histogram
	requests                      *prometheus.CounterVec
	requestDuration               *prometheus.HistogramVec
	sinks                         *prometheus.CounterVec
	sinkDuration                  *prometheus.HistogramVec
	stages                        *prometheus.HistogramVec
	stageFailures                 *prometheus.CounterVec
	users                         prometheus.Gauge
	fileSuccess                   prometheus.Gauge
}

func New(r *prometheus.Registry) *Metrics {
	m := &Metrics{
		namespace:       prometheus.NewDesc("k8s_namespace_sensitive_access_users_count", "Distinct monitored users with matching grants, including usable named grants.", []string{"namespace", "verb", "resource"}, nil),
		cluster:         prometheus.NewDesc("k8s_cluster_wide_sensitive_access_users_count", "Distinct monitored users with all-namespace or cluster-scoped matching grants.", []string{"resource", "verb"}, nil),
		last:            prometheus.NewDesc("access_monitor_last_successful_observation_timestamp_seconds", "Last complete resolved observation; unchanged on failure.", nil, nil),
		has:             prometheus.NewDesc("access_monitor_has_observation", "Whether any complete observation has succeeded.", nil, nil),
		collections:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "access_monitor_collections_total", Help: "Collection outcomes."}, []string{"result"}),
		duration:        prometheus.NewHistogram(prometheus.HistogramOpts{Name: "access_monitor_collection_duration_seconds", Help: "Collection duration.", Buckets: prometheus.DefBuckets}),
		requests:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "access_monitor_kubernetes_requests_total", Help: "Kubernetes request attempts."}, []string{"operation", "result"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "access_monitor_kubernetes_request_duration_seconds", Help: "Kubernetes attempt duration."}, []string{"operation"}),
		sinks:           prometheus.NewCounterVec(prometheus.CounterOpts{Name: "access_monitor_output_total", Help: "Output attempts by sink and result."}, []string{"sink", "result"}),
		sinkDuration:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "access_monitor_output_duration_seconds", Help: "Output write duration."}, []string{"sink"}),
		stages:          prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "access_monitor_stage_duration_seconds", Help: "Processing stage duration."}, []string{"stage"}),
		stageFailures:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "access_monitor_stage_failures_total", Help: "Failed processing stages."}, []string{"stage"}),
		users:           prometheus.NewGauge(prometheus.GaugeOpts{Name: "access_monitor_loaded_users", Help: "Validated monitored inventory size."}),
		fileSuccess:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "access_monitor_last_full_file_report_timestamp_seconds", Help: "Last complete report accepted by file sink; not delivery acknowledgement."}),
	}
	r.MustRegister(m, m.collections, m.duration, m.requests, m.requestDuration, m.sinks, m.sinkDuration, m.stages, m.stageFailures, m.users, m.fileSuccess, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.namespace
	ch <- m.cluster
	ch <- m.last
	ch <- m.has
}
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	has := 0.0
	if !m.at.IsZero() {
		has = 1
		ch <- prometheus.MustNewConstMetric(m.last, prometheus.GaugeValue, float64(m.at.Unix()))
		for k, v := range m.counts.Namespace {
			ch <- prometheus.MustNewConstMetric(m.namespace, prometheus.GaugeValue, float64(v), k.Namespace, k.Verb, k.Resource)
		}
		for k, v := range m.counts.Cluster {
			ch <- prometheus.MustNewConstMetric(m.cluster, prometheus.GaugeValue, float64(v), k.Resource, k.Verb)
		}
	}
	ch <- prometheus.MustNewConstMetric(m.has, prometheus.GaugeValue, has)
}
func (m *Metrics) Publish(c domain.Counts, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts = domain.Counts{Namespace: map[domain.NamespaceKey]int{}, Cluster: map[domain.PermissionKey]int{}}
	for k, v := range c.Namespace {
		m.counts.Namespace[k] = v
	}
	for k, v := range c.Cluster {
		m.counts.Cluster[k] = v
	}
	m.at = at
}
func (m *Metrics) SetReady(b bool) { m.ready.Store(b) }
func (m *Metrics) Ready() bool     { return m.ready.Load() }
func (m *Metrics) Collection(result string, d time.Duration) {
	m.collections.WithLabelValues(result).Inc()
	m.duration.Observe(d.Seconds())
}
func (m *Metrics) Request(op, result string, d time.Duration) {
	m.requests.WithLabelValues(op, result).Inc()
	m.requestDuration.WithLabelValues(op).Observe(d.Seconds())
}
func (m *Metrics) Output(sink string, err error, d time.Duration) {
	result := "success"
	if err != nil {
		result = "failure"
	}
	m.sinks.WithLabelValues(sink, result).Inc()
	m.sinkDuration.WithLabelValues(sink).Observe(d.Seconds())
}
func (m *Metrics) Stage(stage string, d time.Duration, err error) {
	m.stages.WithLabelValues(stage).Observe(d.Seconds())
	if err != nil {
		m.stageFailures.WithLabelValues(stage).Inc()
	}
}
func (m *Metrics) SetUsers(n int)          { m.users.Set(float64(n)) }
func (m *Metrics) FileReport(at time.Time) { m.fileSuccess.Set(float64(at.Unix())) }

package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/output"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type fetcher struct{ err error }

func (f *fetcher) Fetch(context.Context) (domain.State, error) { return domain.State{}, f.err }

type failedSink struct{}

func (failedSink) Write([]byte) (int, error) { return 0, errors.New("file failed") }
func TestCompleteObservationAndFailureFreshness(t *testing.T) {
	c, _ := config.LoadFrom(t.TempDir())
	r := prometheus.NewRegistry()
	m := observability.New(r)
	var b bytes.Buffer
	w := output.New(&b, &b)
	f := &fetcher{}
	s := New(c, []domain.User{{Username: "ali", Groups: []string{}}}, domain.Policy{}, f, w, m, zap.NewNop())
	if err := s.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	var rec output.Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Username != "ali" || rec.Grants == nil || rec.EventID == "" || rec.IdentityBasis != "supplied_input_subjects" {
		t.Fatalf("bad record %+v", rec)
	}
	b.Reset()
	f.err = errors.New("dependency failed")
	if err := s.Collect(context.Background()); err == nil {
		t.Fatal("accepted failed fetch")
	}
	if b.Len() != 0 {
		t.Fatal("failed observation emitted successful report")
	}
	families, _ := r.Gather()
	for _, family := range families {
		if family.GetName() == "access_monitor_last_successful_observation_timestamp_seconds" && family.Metric[0].Gauge.GetValue() != float64(rec.ObservedAt.Unix()) {
			t.Fatal("freshness changed on failure")
		}
	}
}
func TestOutputFailureDoesNotInvalidateMetrics(t *testing.T) {
	c, _ := config.LoadFrom(t.TempDir())
	r := prometheus.NewRegistry()
	m := observability.New(r)
	var good bytes.Buffer
	s := New(c, []domain.User{{Username: "ali"}}, domain.Policy{}, &fetcher{}, output.New(failedSink{}, &good), m, zap.NewNop())
	if err := s.Collect(context.Background()); err != nil {
		t.Fatal("output failure misclassified as observation failure", err)
	}
	if good.Len() == 0 {
		t.Fatal("stdout suppressed")
	}
	families, _ := r.Gather()
	for _, f := range families {
		if f.GetName() == "access_monitor_has_observation" {
			if f.Metric[0].Gauge.GetValue() != 1 {
				t.Fatal("lost fresh observation")
			}
			return
		}
	}
	t.Fatal("missing freshness signal")
}

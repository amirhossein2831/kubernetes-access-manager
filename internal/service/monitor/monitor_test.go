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
	"github.com/amirhossein2831/kubernetes-access-manager/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type fetcher struct {
	err error
}

func (f *fetcher) Fetch(context.Context) (domain.State, error) {
	return domain.State{}, f.err
}

type failedSink struct{}

func TestNewRejectsInvalidInputs(t *testing.T) {
	for _, field := range []string{"user", "policy"} {
		t.Run(field, func(t *testing.T) {
			c, err := config.LoadFrom("../../../configs")
			if err != nil {
				t.Fatal(err)
			}
			if field == "user" {
				c.User = `{}`
			} else {
				c.Policy = `{}`
			}
			service, err := New(c, &fetcher{}, logger.NewWriter(&bytes.Buffer{}, &bytes.Buffer{}), zap.NewNop())
			if err == nil || service != nil || !strings.Contains(err.Error(), "invalid "+field+" JSON") {
				t.Fatalf("expected %s validation failure, got %v", field, err)
			}
		})
	}
}

func (failedSink) Write([]byte) (int, error) {
	return 0, errors.New("file failed")
}

func TestCompleteObservationAndFailureFreshness(t *testing.T) {
	c, _ := config.LoadFrom("../../../configs")
	r := prometheus.DefaultGatherer
	var b bytes.Buffer
	w := logger.NewWriter(&b, &b)
	f := &fetcher{}
	c.User = `{"data":[{"internals":{"one":{"username":"ali","groups":[]}}}]}`
	s, err := New(c, f, w, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	var rec domain.Record
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
	c, _ := config.LoadFrom("../../../configs")
	r := prometheus.DefaultGatherer
	var good bytes.Buffer
	c.User = `{"data":[{"internals":{"one":{"username":"ali"}}}]}`
	s, err := New(c, &fetcher{}, logger.NewWriter(failedSink{}, &good), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
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

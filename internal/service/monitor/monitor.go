package monitor

import (
	"context"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/output"
	"go.uber.org/zap"
)

type Fetcher interface {
	Fetch(context.Context) (domain.State, error)
}
type Emitter interface{ Write(any) output.Results }
type Service struct {
	cfg     *config.Config
	users   []domain.User
	policy  domain.Policy
	fetcher Fetcher
	emitter Emitter
	metrics *observability.Metrics
	log     *zap.Logger
}

func New(c *config.Config, users []domain.User, p domain.Policy, f Fetcher, e Emitter, m *observability.Metrics, log *zap.Logger) *Service {
	m.SetUsers(len(users))
	return &Service{c, users, p, f, e, m, log}
}
func (s *Service) Collect(parent context.Context) error {
	start := time.Now().UTC()
	snapshot := output.NewID()
	ctx, cancel := context.WithTimeout(parent, s.cfg.Poll.Timeout)
	defer cancel()
	fail := func(stage string, err error) error {
		s.metrics.Collection("failure", time.Since(start))
		s.log.Error("collection failed", zap.String("snapshot_id", snapshot), zap.String("stage", stage), zap.Error(err))
		return err
	}
	stage := time.Now()
	state, err := s.fetcher.Fetch(ctx)
	s.metrics.Stage("fetch", time.Since(stage), err)
	if err != nil {
		return fail("fetch", err)
	}
	stage = time.Now()
	access, err := domain.Resolve(s.users, state)
	if err == nil {
		err = ctx.Err()
	}
	s.metrics.Stage("resolve", time.Since(stage), err)
	if err != nil {
		return fail("resolve", err)
	}
	stage = time.Now()
	counts := domain.Count(access, s.policy)
	err = ctx.Err()
	s.metrics.Stage("count", time.Since(stage), err)
	if err != nil {
		return fail("count", err)
	}
	observed := time.Now().UTC()
	s.metrics.Publish(counts, observed)
	s.metrics.Collection("success", time.Since(start))
	allFile := true
	stage = time.Now()
	for _, a := range access {
		if parent.Err() != nil {
			allFile = false
			break
		}
		r := s.emitter.Write(output.Record{SchemaVersion: "v1", EventType: "access_snapshot", EventID: output.EventID(snapshot, a.Username), SnapshotID: snapshot, ObservedAt: observed, StartedAt: start, ExpectedUsers: len(s.users), IdentityBasis: "supplied_input_subjects", UserAccess: a})
		if r.File != nil {
			allFile = false
		}
	}
	s.metrics.Stage("output", time.Since(stage), nil)
	if allFile {
		s.metrics.FileReport(observed)
	}
	return nil
}

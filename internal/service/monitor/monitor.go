package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/logger"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability/metric"
	"go.uber.org/zap"
)

type Fetcher interface {
	Fetch(context.Context) (domain.State, error)
}

type Emitter interface {
	Write(any) logger.Results
}

type AccessMonitor struct {
	cfg     *config.Config
	users   []domain.User
	policy  domain.Policy
	fetcher Fetcher
	emitter Emitter
	log     *zap.Logger
}

func New(c *config.Config, f Fetcher, e Emitter, log *zap.Logger) (*AccessMonitor, error) {
	users, err := domain.ParseUsers(c.User)
	if err != nil {
		return nil, fmt.Errorf("invalid user JSON: %w", err)
	}
	policy, err := domain.ParsePolicy(c.Policy)
	if err != nil {
		return nil, fmt.Errorf("invalid policy JSON: %w", err)
	}
	metric.LoadedUsers.Set(float64(len(users)))
	return &AccessMonitor{
		cfg:     c,
		users:   users,
		policy:  policy,
		fetcher: f,
		emitter: e,
		log:     log,
	}, nil
}

func (s *AccessMonitor) Collect(parent context.Context) error {
	start := time.Now().UTC()
	snapshot := domain.NewID()
	ctx, cancel := context.WithTimeout(parent, s.cfg.Poll.Timeout)
	defer cancel()

	state, err := s.fetch(ctx)
	if err != nil {
		return s.collectionFailed(snapshot, "fetch", start, err)
	}

	access, err := s.resolve(ctx, state)
	if err != nil {
		return s.collectionFailed(snapshot, "resolve", start, err)
	}

	counts, err := s.count(ctx, access)
	if err != nil {
		return s.collectionFailed(snapshot, "count", start, err)
	}

	observed := time.Now().UTC()
	metric.CollectionSucceeded(counts, observed, start)
	s.writeRecords(parent, access, snapshot, start, observed)
	return nil
}

func (s *AccessMonitor) fetch(ctx context.Context) (domain.State, error) {
	start := time.Now()
	state, err := s.fetcher.Fetch(ctx)
	observeStage("fetch", start, err)
	return state, err
}

func (s *AccessMonitor) resolve(ctx context.Context, state domain.State) ([]domain.UserAccess, error) {
	start := time.Now()
	access, err := domain.Resolve(s.users, state)
	if err == nil {
		err = ctx.Err()
	}
	observeStage("resolve", start, err)
	return access, err
}

func (s *AccessMonitor) count(ctx context.Context, access []domain.UserAccess) (domain.Counts, error) {
	start := time.Now()
	counts := domain.Count(access, s.policy)
	err := ctx.Err()
	observeStage("count", start, err)
	return counts, err
}

func observeStage(stage string, start time.Time, err error) {
	metric.StageDuration.WithLabelValues(stage).Observe(time.Since(start).Seconds())
	if err != nil {
		metric.StageFailuresTotal.WithLabelValues(stage).Inc()
	}
}

func (s *AccessMonitor) writeRecords(ctx context.Context, access []domain.UserAccess, snapshot string, start, observed time.Time) {
	allFile := true
	stage := time.Now()
	for _, a := range access {
		if ctx.Err() != nil {
			allFile = false
			break
		}
		r := s.emitter.Write(domain.Record{
			SchemaVersion: "v1",
			EventType:     "access_snapshot",
			EventID:       domain.EventID(snapshot, a.Username),
			SnapshotID:    snapshot,
			ObservedAt:    observed,
			StartedAt:     start,
			ExpectedUsers: len(s.users),
			IdentityBasis: "supplied_input_subjects",
			UserAccess:    a,
		})
		if r.File != nil {
			allFile = false
		}
	}
	metric.StageDuration.WithLabelValues("output").Observe(time.Since(stage).Seconds())
	if allFile {
		metric.LastFullFileReport.Set(float64(observed.Unix()))
	}
}

func (s *AccessMonitor) collectionFailed(snapshot, stage string, start time.Time, err error) error {
	metric.CollectionFailed(start)
	s.log.Error("collection failed", zap.String("snapshot_id", snapshot), zap.String("stage", stage), zap.Error(err))
	return err
}

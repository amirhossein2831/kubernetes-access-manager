package monitor

import (
	"context"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/config"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/logger"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/observability/metric"
	"github.com/amirhossein2831/kubernetes-access-manager/internal/pkg/client/http/kubernetes"
	"go.uber.org/zap"
)

type Fetcher interface {
	Fetch(context.Context) (domain.State, error)
}
type Emitter interface {
	Write(any) logger.Results
}
type Service struct {
	cfg     *config.Config
	users   []domain.User
	policy  domain.Policy
	fetcher Fetcher
	emitter Emitter
	log     *zap.Logger
}

func New(c *config.Config, users []domain.User, p domain.Policy, f Fetcher, e Emitter, log *zap.Logger) *Service {
	metric.LoadedUsers.Set(float64(len(users)))
	return &Service{
		c,
		users,
		p,
		f,
		e,
		log,
	}
}

func Init(c *config.Config, client *kubernetes.Client, w *logger.Writer, log *zap.Logger) *Service {
	users := make([]domain.User, 0, len(c.User))
	for _, user := range c.User {
		users = append(users, domain.User{
			Username: user.Username,
			Groups:   append([]string{}, user.Groups...),
		})
	}
	p := domain.Policy{
		Namespaces: append([]string{}, c.Policy.Namespaces...),
	}
	for _, permission := range c.Policy.Permissions {
		p.Permissions = append(p.Permissions, domain.Permission{
			APIGroup: permission.APIGroup,
			Resource: permission.Resource,
			Verbs:    append([]string{}, permission.Verbs...),
		})
	}
	return New(c, users, p, client, w, log)
}

func (s *Service) Collect(parent context.Context) error {
	start := time.Now().UTC()
	snapshot := domain.NewID()
	ctx, cancel := context.WithTimeout(parent, s.cfg.Poll.Timeout)
	defer cancel()
	fail := func(stage string, err error) error {
		metric.CollectionsTotal.WithLabelValues("failure").Inc()
		metric.CollectionDuration.Observe(time.Since(start).Seconds())
		s.log.Error("collection failed", zap.String("snapshot_id", snapshot), zap.String("stage", stage), zap.Error(err))
		return err
	}
	stage := time.Now()
	state, err := s.fetcher.Fetch(ctx)
	metric.StageDuration.WithLabelValues("fetch").Observe(time.Since(stage).Seconds())
	if err != nil {
		metric.StageFailuresTotal.WithLabelValues("fetch").Inc()
		return fail("fetch", err)
	}
	stage = time.Now()
	access, err := domain.Resolve(s.users, state)
	if err == nil {
		err = ctx.Err()
	}
	metric.StageDuration.WithLabelValues("resolve").Observe(time.Since(stage).Seconds())
	if err != nil {
		metric.StageFailuresTotal.WithLabelValues("resolve").Inc()
		return fail("resolve", err)
	}
	stage = time.Now()
	counts := domain.Count(access, s.policy)
	err = ctx.Err()
	metric.StageDuration.WithLabelValues("count").Observe(time.Since(stage).Seconds())
	if err != nil {
		metric.StageFailuresTotal.WithLabelValues("count").Inc()
		return fail("count", err)
	}
	observed := time.Now().UTC()
	metric.NamespaceSensitiveAccessUsers.Reset()
	metric.ClusterSensitiveAccessUsers.Reset()
	for key, count := range counts.Namespace {
		metric.NamespaceSensitiveAccessUsers.WithLabelValues(key.Namespace, key.Verb, key.Resource).Set(float64(count))
	}
	for key, count := range counts.Cluster {
		metric.ClusterSensitiveAccessUsers.WithLabelValues(key.Resource, key.Verb).Set(float64(count))
	}
	metric.LastSuccessfulObservation.Set(float64(observed.Unix()))
	metric.HasObservation.Set(1)
	metric.CollectionsTotal.WithLabelValues("success").Inc()
	metric.CollectionDuration.Observe(time.Since(start).Seconds())
	allFile := true
	stage = time.Now()
	for _, a := range access {
		if parent.Err() != nil {
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
	return nil
}

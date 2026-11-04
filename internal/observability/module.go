package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

var Module = fx.Module("observability",
	fx.Provide(
		func() prometheus.Gatherer { return prometheus.DefaultGatherer },
	),
)

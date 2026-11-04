package jobs

import "go.uber.org/fx"

var Module = fx.Module("jobs",
	fx.Invoke(Init),
)

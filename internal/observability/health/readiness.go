package health

import "sync/atomic"

// Ready tracks lifecycle readiness, independently of metrics and collection success.
var Ready atomic.Bool

// Package timers is here just to register the k6/x/events module
package timers

import (
	"github.com/grafana/xk6-timers/timers"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/timers", new(timers.RootModule))
}

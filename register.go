// Package events is here just to register the k6/x/events module
package events

import (
	"github.com/mstoykov/xk6-events/events"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/events", new(events.RootModule))
}

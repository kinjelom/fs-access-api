package metrics

import (
	"fs-access-api/internal/app/ports"
	"log"
)

type FakeActionMetrics struct {
}

// Enforce compile-time conformance to the interface
var _ ports.ActionMetrics = (*FakeActionMetrics)(nil)

// OnActionDone updates all metrics for a single probe result.
func (m *FakeActionMetrics) OnActionDone(ma ports.MeasuredAction) {
	mal := ma.Labels()
	log.Printf("FakeActionMetrics.OnActionDone: %v", mal)
}

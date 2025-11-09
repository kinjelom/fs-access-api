package metrics

import (
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type AuthzActionMetrics struct {
	cfg                     config.MetricsContext
	BuildInfo               *prometheus.GaugeVec
	ActionDurationHistogram *prometheus.HistogramVec
	UserActionsTotal        *prometheus.CounterVec
}

// Enforce compile-time conformance to the interface
var _ ports.ActionMetrics = (*AuthzActionMetrics)(nil)

func NewAuthzActionMetrics(programName, programVersion string, cfg config.MetricsContext, reg prometheus.Registerer) (*AuthzActionMetrics, error) {
	constLabels := prometheus.Labels{
		"environment":     cfg.Environment,
		"program_name":    programName,
		"program_version": programVersion,
	}

	var actionLabels = []string{string(ports.MALabelAction), string(ports.MALabelResult)}
	var userActionLabels = []string{string(ports.MALabelAction), string(ports.MALabelUsername), string(ports.MALabelResult)}
	pa := promauto.With(reg)
	m := &AuthzActionMetrics{
		cfg: cfg,
		BuildInfo: pa.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   cfg.Namespace,
				Name:        "build_info",
				Help:        "Build information for this binary; constant value 1.",
				ConstLabels: constLabels,
			},
			[]string{}, // no dynamic labels
		),

		// Aggregates duration distribution over time.
		ActionDurationHistogram: pa.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   cfg.Namespace,
				Name:        "authz_action_duration_seconds",
				Help:        "Distribution of authorization action durations in seconds.",
				Buckets:     []float64{0.010, 0.100, 0.500, 1.0, 3.0, 5.0, 10.0},
				ConstLabels: prometheus.Labels{"environment": cfg.Environment},
			},
			actionLabels,
		),

		UserActionsTotal: pa.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   cfg.Namespace,
				Name:        "authz_user_actions_total",
				Help:        "Total number of authorization-related actions partitioned by action, username and result.",
				ConstLabels: prometheus.Labels{"environment": cfg.Environment},
			},
			userActionLabels,
		),
	}

	m.BuildInfo.With(nil).Set(1)
	return m, nil
}

// OnActionDone updates all metrics for a single probe result.
func (m *AuthzActionMetrics) OnActionDone(ma ports.MeasuredAction) {
	mal := ma.Labels()
	labels := prometheus.Labels{
		string(ports.MALabelAction): mal[ports.MALabelAction],
		string(ports.MALabelResult): mal[ports.MALabelResult],
	}
	userLabels := prometheus.Labels{
		string(ports.MALabelAction):   mal[ports.MALabelAction],
		string(ports.MALabelUsername): mal[ports.MALabelUsername],
		string(ports.MALabelResult):   mal[ports.MALabelResult],
	}
	m.ActionDurationHistogram.With(labels).Observe(ma.Duration())
	m.UserActionsTotal.With(userLabels).Inc()
}

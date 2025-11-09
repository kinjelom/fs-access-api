package ports

type ActionMetrics interface {
	OnActionDone(ma MeasuredAction)
}

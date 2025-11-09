package metrics

import (
	"errors"
	"fs-access-api/internal/app/ports"
	"time"
)

type AuthzAction struct {
	start           time.Time
	Action          string
	Username        string
	Result          string
	DurationFloat64 float64
}

// Enforce compile-time conformance to the interface
var _ ports.MeasuredAction = (*AuthzAction)(nil)

func (a *AuthzAction) Duration() float64 {
	return a.DurationFloat64
}

func NewAuthzAction(action, username string) *AuthzAction {
	return &AuthzAction{
		start:    time.Now(),
		Action:   action,
		Username: username,
		Result:   "unknown",
	}
}

func (a *AuthzAction) Done(result ports.MeasuredActionResult) ports.MeasuredAction {
	a.Result = string(result)
	a.DurationFloat64 = time.Since(a.start).Seconds()
	return a
}
func (a *AuthzAction) DoneFromError(err error) ports.MeasuredAction {
	a.Result = string(measuredFromError(err))
	a.DurationFloat64 = time.Since(a.start).Seconds()
	return a
}

func (a *AuthzAction) Labels() map[ports.MeasuredActionLabel]string {
	return map[ports.MeasuredActionLabel]string{
		ports.MALabelAction:   a.Action,
		ports.MALabelUsername: a.Username,
		ports.MALabelResult:   a.Result,
	}
}

func measuredFromError(err error) ports.MeasuredActionResult {
	if err == nil {
		return ports.MAResultSuccess
	}
	switch {
	case errors.Is(err, ports.ErrNotFound):
		return ports.MAResultNotFound
	case errors.Is(err, ports.ErrInvalidCredentials),
		errors.Is(err, ports.ErrInvalidInput):
		return ports.MAResultForbiddenUser
	case errors.Is(err, ports.ErrLockedUser):
		return ports.MAResultLockedUser
	default:
		return ports.MAResultFailure
	}
}

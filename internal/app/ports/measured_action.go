package ports

type MeasuredActionLabel string
type MeasuredActionResult string

const (
	MALabelAction                 MeasuredActionLabel  = "action"
	MALabelUsername               MeasuredActionLabel  = "username"
	MALabelResult                 MeasuredActionLabel  = "result"
	MAResultSuccess               MeasuredActionResult = "success"
	MAResultFailure               MeasuredActionResult = "failure"
	MAResultUnauthorizedApiClient MeasuredActionResult = "api-client-unauthorized"
	MAResultNotFound              MeasuredActionResult = "not-found"
	MAResultForbiddenUser         MeasuredActionResult = "forbidden"
	MAResultLockedUser            MeasuredActionResult = "locked"
)

type MeasuredAction interface {
	Done(result MeasuredActionResult) MeasuredAction
	Duration() float64
	Labels() map[MeasuredActionLabel]string
}

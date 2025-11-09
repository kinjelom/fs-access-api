package ports

import (
	"net/http"
)

type Authenticator interface {
	WithAuthChi(handler http.Handler) http.Handler
	Verify(request *http.Request) error
	Supports(request *http.Request) bool
}

//go:build windows

package app

import "fmt"

// Dummy no-op for Windows
func CreatePIDFile(path string) (func(), error) {
	return func() {}, fmt.Errorf("creating PID file is not available on Windows")
}

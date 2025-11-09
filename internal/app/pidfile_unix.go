//go:build unix

package app

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// CreatePIDFile creates/locks a PID file and writes os.Getpid().
// It returns a cleanup func that unlocks and removes the file.
func CreatePIDFile(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	// Open or create with RW; we’ll lock it before writing.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	// Try to acquire an exclusive, non-blocking lock.
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		// Another process holds the lock ⇒ already running.
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, fmt.Errorf("another instance appears to be running (pidfile locked: %s)", path)
		}
		return nil, fmt.Errorf("flock %s: %w", path, err)
	}

	// Truncate and write our PID.
	if err := f.Truncate(0); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("truncate %s: %w", path, err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("seek %s: %w", path, err)
	}
	if _, err := f.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("write pid to %s: %w", path, err)
	}
	// Keep the file open while locked; POSIX locks are per-fd.

	// Setup signal cleanup.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	cleanup := func() {
		// Best effort: unlock, close, remove.
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
		_ = os.Remove(path)
	}

	go func() {
		<-signals
		cleanup()
		os.Exit(0)
	}()

	return cleanup, nil
}

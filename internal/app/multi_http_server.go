package app

import (
	"context"
	"errors"
	"fmt"
	"fs-access-api/internal/app/config"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type MultiHTTPServer struct {
	cfg          config.HttpServerConfig
	handler      http.Handler
	tcp          *http.Server
	unix         *http.Server
	unixListener net.Listener
	serveErr     chan error
}

func NewMultiHTTPServer(cfg config.HttpServerConfig, handler http.Handler) (*MultiHTTPServer, error) {
	s := &MultiHTTPServer{
		cfg:      cfg,
		handler:  handler,
		serveErr: make(chan error, 2),
	}
	if cfg.ListenAddress != "" {
		s.initTCP()
	}
	if cfg.UnixSocketPath != "" {
		if err := s.initUnix(); err != nil {
			return nil, err
		}
	}
	if s.tcp == nil && s.unix == nil {
		return nil, fmt.Errorf("no listeners configured")
	}
	return s, nil
}

func (s *MultiHTTPServer) initTCP() {
	s.tcp = &http.Server{
		Addr:              s.cfg.ListenAddress,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 16, // 64 KB
	}
}

func (s *MultiHTTPServer) initUnix() error {
	p := s.cfg.UnixSocketPath
	if p == "" {
		return fmt.Errorf("empty UnixSocketPath")
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o775); err != nil {
		return fmt.Errorf("failed to mkdir %s: %w", filepath.Dir(p), err)
	}

	// Remove stale socket if present.
	_ = os.Remove(p)

	// Bind explicitly as Unix listener.
	ua := &net.UnixAddr{Name: p, Net: "unix"}
	ln, err := net.ListenUnix("unix", ua)
	if err != nil {
		return fmt.Errorf("failed to listen unix %s: %w", p, err)
	}

	// Ensure the socket path is cleaned on close (Go will unlink on Close).
	// This does not affect visibility while running.
	ln.SetUnlinkOnClose(true)

	// Set permissions on the socket file (after it exists).
	if err := os.Chmod(p, 0o660); err != nil {
		_ = ln.Close()
		_ = os.Remove(p)
		return fmt.Errorf("failed to chmod %s: %w", p, err)
	}

	// Sanity check: confirm it is a socket node on disk.
	if fi, err := os.Lstat(p); err != nil {
		_ = ln.Close()
		return fmt.Errorf("failed to stat %s (was it removed by something?): %w", p, err)
	} else if fi.Mode()&os.ModeSocket == 0 {
		_ = ln.Close()
		return fmt.Errorf("%s exists but is not a socket (mode=%v)", p, fi.Mode())
	}

	s.unixListener = ln
	s.unix = &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	return nil
}

// Start launches servers in goroutines. Use WaitAndShutdown to block.
func (s *MultiHTTPServer) Start() {
	log.Printf("Starting HTTP server '%s'", s.cfg.Banner)
	if s.tcp != nil {
		go func() {
			log.Printf("listening on TCP %s", s.cfg.ListenAddress)
			if err := s.tcp.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.serveErr <- fmt.Errorf("tcp: %w", err)
			}
		}()
	}
	if s.unix != nil && s.unixListener != nil {
		go func() {
			log.Printf("listening on Unix socket %s", s.cfg.UnixSocketPath)
			if err := s.unix.Serve(s.unixListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.serveErr <- fmt.Errorf("unix: %w", err)
			}
		}()
	}
}

// WaitAndShutdown blocks on SIGINT/SIGTERM or first serve error, then shuts down.
func (s *MultiHTTPServer) WaitAndShutdown() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
	case err := <-s.serveErr:
		log.Printf("server error: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if s.tcp != nil {
		if err := s.tcp.Shutdown(shutdownCtx); err == nil {
			log.Printf("TCP connection was gracefully shut down")
		} else {
			log.Printf("TCP shutdown error: %v", err)
		}
	}
	if s.unix != nil {
		if err := s.unix.Shutdown(shutdownCtx); err == nil {
			log.Printf("Unix connection was gracefully shut down")
		} else {
			log.Printf("Unix shutdown error: %v", err)
		}
		if s.unixListener != nil {
			_ = s.unixListener.Close()
		}
		if s.cfg.UnixSocketPath != "" {
			_ = os.Remove(s.cfg.UnixSocketPath)
		}
	}
	log.Printf("Shutdown HTTP server '%s'", s.cfg.Banner)
}

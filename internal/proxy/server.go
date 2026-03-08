// Package proxy provides a reverse proxy server for forwarding API requests.
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// ServerConfig holds the configuration for a proxy Server.
type ServerConfig struct {
	Port        int
	UpstreamURL string
}

// Server wraps httputil.ReverseProxy with modeltap's configuration.
type Server struct {
	proxy    *httputil.ReverseProxy
	upstream *url.URL
	port     int
	server   *http.Server
}

// NewServer creates a new proxy Server from the given config.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.UpstreamURL == "" {
		return nil, fmt.Errorf("upstream URL is required")
	}
	if cfg.Port <= 0 {
		return nil, fmt.Errorf("port must be positive")
	}

	upstream, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("parsing upstream URL: %w", err)
	}

	if upstream.Scheme == "" || upstream.Host == "" {
		return nil, fmt.Errorf("upstream URL must have a scheme and host")
	}

	rp := httputil.NewSingleHostReverseProxy(upstream)

	// Customize the Director to preserve path and set the Host header
	// to the upstream's host.
	originalDirector := rp.Director
	rp.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = upstream.Host
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: rp,
	}

	return &Server{
		proxy:    rp,
		upstream: upstream,
		port:     cfg.Port,
		server:   httpServer,
	}, nil
}

// Start begins listening and serving HTTP requests. It blocks until the
// server is shut down. Returns http.ErrServerClosed on graceful shutdown.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server, waiting for active connections
// to complete within the given context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Port returns the configured port.
func (s *Server) Port() int {
	return s.port
}

// UpstreamURL returns the configured upstream URL.
func (s *Server) UpstreamURL() string {
	return s.upstream.String()
}

// Handler returns the underlying HTTP handler (the reverse proxy).
// This is useful for testing without starting a full server.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

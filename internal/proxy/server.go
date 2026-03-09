// Package proxy provides a reverse proxy server for forwarding API requests.
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// ServerConfig holds the configuration for a proxy Server.
type ServerConfig struct {
	Port              int
	UpstreamURL       string
	Store             storage.Store
	Registry          *provider.Registry
	ProviderUpstreams map[string]string // provider name -> upstream URL
	Pricing           *config.PricingTable
}

// Server wraps httputil.ReverseProxy with modeltap's configuration.
type Server struct {
	proxy             *httputil.ReverseProxy
	upstream          *url.URL
	providerUpstreams map[string]*url.URL
	registry          *provider.Registry
	port              int
	server            *http.Server
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

	// Parse provider-specific upstream URLs.
	providerUpstreams := make(map[string]*url.URL)
	for name, rawURL := range cfg.ProviderUpstreams {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parsing upstream URL for provider %q: %w", name, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("upstream URL for provider %q must have a scheme and host", name)
		}
		providerUpstreams[name] = u
	}

	rp := httputil.NewSingleHostReverseProxy(upstream)

	// Customize the Director to detect the provider and route to the
	// correct upstream. Falls back to the default upstream if no
	// provider-specific upstream is configured.
	rp.Director = func(req *http.Request) {
		target := upstream
		if cfg.Registry != nil {
			if p := cfg.Registry.Detect(req); p != nil {
				if u, ok := providerUpstreams[p.Name()]; ok {
					target = u
				}
			}
		}
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		if _, ok := req.Header["User-Agent"]; !ok {
			// Prevent Go's default User-Agent from being sent.
			req.Header.Set("User-Agent", "")
		}
	}

	// Wrap the proxy with capture middleware if Store and Registry are provided.
	var handler http.Handler = rp
	if cfg.Store != nil && cfg.Registry != nil {
		capture := NewCaptureMiddleware(cfg.Store, cfg.Registry, cfg.Pricing)
		handler = capture.Wrap(rp)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	return &Server{
		proxy:             rp,
		upstream:          upstream,
		providerUpstreams: providerUpstreams,
		registry:          cfg.Registry,
		port:              cfg.Port,
		server:            httpServer,
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

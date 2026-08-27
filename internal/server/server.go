package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	// readHeaderTimeout bounds how long we wait for a client to send request
	// headers. Requests here are tiny (a URL, maybe some JSON), so any real
	// client finishes well under this; it mainly guards against slow-header
	// (Slowloris-style) connections tying up a handler goroutine.
	readHeaderTimeout = 5 * time.Second

	// readTimeout bounds reading the full request (headers + body). We never
	// accept large payloads (no file uploads), so a short timeout is safe.
	readTimeout = 5 * time.Second

	// writeTimeout bounds writing the response. Responses are a redirect or a
	// short JSON body, so this is generous headroom rather than a real
	// constraint in normal operation.
	writeTimeout = 10 * time.Second

	// idleTimeout bounds how long a keep-alive connection may sit idle
	// between requests before it's closed. 120s is a standard window that
	// lets clients reuse connections without holding them open forever.
	idleTimeout = 120 * time.Second

	// requestTimeout is a per-request context deadline enforced in
	// middleware. A redirect lookup or short-link creation should never
	// legitimately take longer; this catches a hung downstream call (DB,
	// cache) and fails the request instead of leaking a goroutine.
	requestTimeout = 5 * time.Second
)

type Server struct {
	logger *slog.Logger
	http   *http.Server
}

func New(port int, logger *slog.Logger, routes chi.Router) *Server {
	router := chi.NewRouter()
	// ClientIPFromRemoteAddr assumes this server is directly exposed to
	// clients (no reverse proxy/LB in front of it). If a proxy is
	// introduced before this deploys, RemoteAddr will be the proxy's IP —
	// switch to ClientIPFromXFFTrustedProxies(n) instead. REVIEW BEFORE DEPLOY.
	router.Use(middleware.ClientIPFromRemoteAddr)
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(requestTimeout))

	router.Mount("/api", routes)

	logEndpoints(logger, router)

	return &Server{
		logger: logger,
		http: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           router,
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting http server", slog.String("addr", s.http.Addr))

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return s.http.Shutdown(ctx)
}

func logEndpoints(logger *slog.Logger, router chi.Router) {
	err := chi.Walk(router, func(
		method string,
		route string,
		handler http.Handler,
		middlewares ...func(http.Handler) http.Handler,
	) error {
		if route == "/lhealth" || route == "/rhealth" || route == "/docs/*" {
			return nil
		}

		logger.Info(
			"route registered",
			slog.String("method", method),
			slog.String("route", route),
			slog.Int("middlewares", len(middlewares)),
		)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

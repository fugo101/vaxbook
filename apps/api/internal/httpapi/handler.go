package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fugo101/vaxbook/apps/api/internal/identity"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/middleware"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/problem"
)

// BaseURL is where the user API is mounted on the app host.
const BaseURL = "/api/v1"

// MaxBodyBytes caps API request bodies. Endpoints that need more (guest import)
// will raise it per operation.
const MaxBodyBytes = 64 << 10

// Options are the dependencies of the HTTP handler.
type Options struct {
	AppHost  string // host of the user app; requests for other hosts get 404
	Identity *identity.Service
	// Ping checks the database for /healthz.
	Ping   func(context.Context) error
	Logger *slog.Logger
}

// NewHandler returns the root handler of the process (SAD §4: one process, routed
// by Host). Only the app host is wired so far; the admin and MCP hosts will get
// their own sub-handlers and middleware.
func NewHandler(o Options) (http.Handler, error) {
	spec, err := GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("httpapi: load embedded spec: %w", err)
	}
	ops, err := indexOperations(spec, BaseURL)
	if err != nil {
		return nil, err
	}

	app := http.NewServeMux()
	strict := NewStrictHandlerWithOptions(&server{identity: o.Identity}, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				problem.Write(w, http.StatusRequestEntityTooLarge, "request body is too large")
				return
			}
			problem.Write(w, http.StatusBadRequest, "malformed request body")
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			o.Logger.ErrorContext(r.Context(), "handler error", slog.String("route", r.Pattern), slog.String("error", err.Error()))
			problem.Write(w, http.StatusInternalServerError, "")
		},
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseURL:    BaseURL,
		BaseRouter: app,
		// Applied in reverse: MaxBody runs first, then the operation middleware.
		Middlewares: []MiddlewareFunc{
			operationMiddleware(ops, o.Identity, o.Logger),
			middleware.MaxBody(MaxBodyBytes),
		},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			problem.Write(w, http.StatusBadRequest, err.Error())
		},
	})
	app.Handle("GET /healthz", healthz(o.Ping, o.Logger)) // host patterns win over the root's
	app.Handle("/", notFound())

	root := http.NewServeMux()
	root.Handle("GET /healthz", healthz(o.Ping, o.Logger))
	// CSRF wraps the app host only: the generated code binds the CSRF header
	// parameter before per-operation middleware runs, so the check must come first.
	root.Handle(o.AppHost+"/", middleware.CSRF(app))
	root.Handle("/", notFound())

	return middleware.Chain(normalizeHost(root), o.Logger), nil
}

// normalizeHost lowercases the Host and drops a trailing dot, so host routing
// treats "VaxBook.Example." like "vaxbook.example". Host names are
// case-insensitive; ServeMux matching is not.
func normalizeHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, port := r.Host, ""
		if h, p, err := net.SplitHostPort(r.Host); err == nil {
			host, port = h, p
		}
		host = strings.ToLower(strings.TrimSuffix(host, "."))
		if port != "" {
			host = net.JoinHostPort(host, port)
		}
		r.Host = host
		next.ServeHTTP(w, r)
	})
}

func notFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		problem.Write(w, http.StatusNotFound, "")
	})
}

// healthz answers 200 when the database answers within two seconds.
func healthz(ping func(context.Context) error, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			logger.ErrorContext(r.Context(), "healthz: database", slog.String("error", err.Error()))
			problem.Write(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("ok\n"))
	})
}

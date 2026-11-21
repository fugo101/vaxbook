// Package middleware holds the cross-cutting HTTP middleware of the vaxbook binary.
//
// The order is fixed by Chain:
//
//	Recover → RequestID → AccessLog → (mux) → per-operation middleware
//
// Recover is outermost so that a panic anywhere, including in other middleware,
// becomes a 500 problem response instead of a dropped connection.
package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"

	"github.com/fugo101/vaxbook/apps/api/internal/platform/logging"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/problem"
)

// RequestIDHeader carries the request id back to the client.
const RequestIDHeader = "X-Request-Id"

// CSRFHeader must be "1" on every write request (SAD §8).
const CSRFHeader = "X-VaxBook-CSRF"

// Chain wraps h with Recover, RequestID and AccessLog, outermost first.
func Chain(h http.Handler, logger *slog.Logger) http.Handler {
	return Recover(logger)(RequestID(AccessLog(logger)(h)))
}

// Recover turns a panic into a 500 problem response and logs the stack trace.
// The log line carries the request id but nothing from the request itself.
// http.ErrAbortHandler is re-raised, as net/http expects.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := wrap(w)
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				logger.ErrorContext(r.Context(), "panic",
					slog.String("request_id", w.Header().Get(RequestIDHeader)),
					slog.String("panic", fmt.Sprint(v)),
					slog.String("stack", string(debug.Stack())))
				if !sw.wroteHeader {
					problem.Write(sw, http.StatusInternalServerError, "")
				}
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

// RequestID gives every request a fresh id, in the context (for logs) and in the
// X-Request-Id response header. Incoming ids are ignored: the header is client
// controlled and would let anyone write arbitrary text into the logs.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.Must(uuid.NewV7()).String()
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(logging.WithRequestID(r.Context(), id)))
	})
}

// AccessLog writes one line per request: method, route pattern, status and
// duration. It never logs the raw path, query, headers or body (SAD §9). A request
// that panics is logged with status 500 before the panic continues to Recover.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := wrap(w)
			defer func() {
				status := sw.status
				v := recover()
				if v != nil {
					status = http.StatusInternalServerError
				}
				route := r.Pattern
				if route == "" {
					route = "unmatched"
				}
				logger.InfoContext(r.Context(), "http request",
					slog.String("method", r.Method),
					slog.String("route", route),
					slog.Int("status", status),
					slog.Duration("duration", time.Since(start)))
				if v != nil {
					panic(v)
				}
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

// MaxBody caps the request body at n bytes.
func MaxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF rejects write requests that do not carry X-VaxBook-CSRF: 1 (SAD §8). A
// cross-site form or simple request cannot set a custom header, and a cross-site
// fetch that sets it needs a CORS preflight, which this API never grants.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get(CSRFHeader) != "1" {
				problem.Write(w, http.StatusForbidden, "missing "+CSRFHeader+" header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// statusWriter records the status code and whether headers were sent.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrap(w http.ResponseWriter) *statusWriter {
	if sw, ok := w.(*statusWriter); ok {
		return sw
	}
	return &statusWriter{ResponseWriter: w, status: http.StatusOK}
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

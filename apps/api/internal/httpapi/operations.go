package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/fugo101/vaxbook/apps/api/internal/identity"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/problem"
)

// operation is what the spec says about one route.
type operation struct {
	route   *routers.Route
	secured bool
}

// operations indexes the spec's operations by the ServeMux pattern the generated
// code registers for them ("METHOD /base/path"), so the middleware can find the
// operation from r.Pattern without a second router.
type operations map[string]operation

func indexOperations(spec *openapi3.T, baseURL string) (operations, error) {
	ops := operations{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			secured := len(spec.Security) > 0
			if op.Security != nil {
				secured = len(*op.Security) > 0
			}
			ops[method+" "+baseURL+path] = operation{
				route:   &routers.Route{Spec: spec, Path: path, PathItem: item, Method: method, Operation: op},
				secured: secured,
			}
		}
	}
	if len(ops) == 0 {
		return nil, errors.New("httpapi: spec has no operations")
	}
	return ops, nil
}

type authKey struct{}

type auth struct {
	session identity.Session
	token   string
}

// authFrom returns the authentication set by the operation middleware. It panics
// if the operation is not secured in the spec, which is a programming error.
func authFrom(ctx context.Context) auth {
	a, ok := ctx.Value(authKey{}).(auth)
	if !ok {
		panic("httpapi: no authenticated session in context; is the operation missing `security` in openapi.yaml?")
	}
	return a
}

// operationMiddleware runs after routing, for every generated operation:
//  1. if the spec gives the operation a security requirement, it requires a live
//     session (401 otherwise) and puts it in the context;
//  2. it validates the request against the spec (400, or 413 for a large body).
func operationMiddleware(ops operations, ids *identity.Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, ok := ops[r.Pattern]
			if !ok {
				// Every generated route is in the spec, so this is a wiring bug.
				logger.ErrorContext(r.Context(), "route not found in spec", slog.String("route", r.Pattern))
				problem.Write(w, http.StatusInternalServerError, "")
				return
			}

			if op.secured {
				a, ok := authenticate(w, r, ids, logger)
				if !ok {
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), authKey{}, a))
			}

			params := map[string]string{}
			for _, p := range op.route.Operation.Parameters {
				if p.Value != nil && p.Value.In == openapi3.ParameterInPath {
					params[p.Value.Name] = r.PathValue(p.Value.Name)
				}
			}
			err := openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: params,
				Route:      op.route,
				Options: &openapi3filter.Options{
					// Authentication is done above; the cookie scheme is not re-checked here.
					AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
				},
			})
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					problem.Write(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body is larger than %d bytes", tooLarge.Limit))
					return
				}
				problem.Write(w, http.StatusBadRequest, validationDetail(err))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// authenticate reads the session cookie and resolves it. On failure it writes the
// response and returns false. A renewed session re-issues the cookie so the
// browser keeps it as long as the server does.
func authenticate(w http.ResponseWriter, r *http.Request, ids *identity.Service, logger *slog.Logger) (auth, bool) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		problem.Write(w, http.StatusUnauthorized, "sign in required")
		return auth{}, false
	}
	sess, err := ids.Authenticate(r.Context(), c.Value)
	if errors.Is(err, identity.ErrNoSession) {
		w.Header().Add("Set-Cookie", sessionCookie("", -1))
		problem.Write(w, http.StatusUnauthorized, "sign in required")
		return auth{}, false
	}
	if err != nil {
		logger.ErrorContext(r.Context(), "authenticate", slog.String("error", err.Error()))
		problem.Write(w, http.StatusInternalServerError, "")
		return auth{}, false
	}
	if sess.Renewed {
		w.Header().Add("Set-Cookie", sessionCookie(c.Value, identity.SessionIdleTTL))
	}
	return auth{session: sess, token: c.Value}, true
}

// validationDetail turns a kin-openapi error into a short message for the client.
func validationDetail(err error) string {
	msg := err.Error()
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) && reqErr.Err != nil {
		msg = reqErr.Error()
	}
	msg = strings.ReplaceAll(msg, "\n", " ")
	const limit = 300
	if len(msg) > limit {
		msg = msg[:limit] + "…"
	}
	return msg
}

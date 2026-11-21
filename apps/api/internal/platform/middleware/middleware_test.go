package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/apps/api/internal/platform/middleware"
)

func newLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewJSONHandler(buf, nil)), buf
}

func TestPanicBecomesProblem500(t *testing.T) {
	logger, logs := newLogger()
	h := middleware.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}), logger)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x?email=secret@example.com", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var p struct{ Status int }
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Status != 500 {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	id := rec.Header().Get(middleware.RequestIDHeader)
	if id == "" {
		t.Fatal("no request id on the 500")
	}
	out := logs.String()
	for _, want := range []string{`"msg":"panic"`, `"panic":"boom"`, `"stack":`, id, `"status":500`} {
		if !strings.Contains(out, want) {
			t.Errorf("logs missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret@example.com") {
		t.Errorf("logs contain the query string:\n%s", out)
	}
}

func TestPanicAfterWritingAbortsTheConnection(t *testing.T) {
	logger, logs := newLogger()
	h := middleware.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partial":`))
		panic("late")
	}), logger)

	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
				t.Fatalf("recovered %v, want http.ErrAbortHandler so net/http drops the connection", err)
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()

	out := logs.String()
	for _, want := range []string{`"panic":"late"`, `"status":200`, `"aborted":true`} {
		if !strings.Contains(out, want) {
			t.Errorf("logs missing %s:\n%s", want, out)
		}
	}
}

// Through a real server the client must see an error, not a truncated 200.
func TestPanicAfterWritingIsVisibleToTheClient(t *testing.T) {
	logger, _ := newLogger()
	srv := httptest.NewServer(middleware.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partial":`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic("late")
	}), logger))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL)
	if err == nil {
		_, err = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("client read a complete response; want a broken connection")
	}
}

func TestAbortHandlerIsReRaised(t *testing.T) {
	logger, _ := newLogger()
	h := middleware.Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if err, ok := recover().(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", err)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestCSRF(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := middleware.CSRF(ok)
	cases := []struct {
		method, header string
		want           int
	}{
		{http.MethodGet, "", http.StatusNoContent},
		{http.MethodHead, "", http.StatusNoContent},
		{http.MethodPost, "", http.StatusForbidden},
		{http.MethodPost, "true", http.StatusForbidden},
		{http.MethodPost, "1", http.StatusNoContent},
		{http.MethodDelete, "", http.StatusForbidden},
		{http.MethodPatch, "1", http.StatusNoContent},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, "/", nil)
		if c.header != "" {
			r.Header.Set(middleware.CSRFHeader, c.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != c.want {
			t.Errorf("%s with %q: %d, want %d", c.method, c.header, rec.Code, c.want)
		}
	}
}

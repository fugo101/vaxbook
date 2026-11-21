package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fugo101/vaxbook/apps/api/internal/httpapi"
	"github.com/fugo101/vaxbook/apps/api/internal/identity"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/clock"
	"github.com/fugo101/vaxbook/apps/api/internal/store"
	"github.com/fugo101/vaxbook/apps/api/internal/store/storetest"
)

var pg *storetest.Server

func TestMain(m *testing.M) { os.Exit(storetest.Main(m, &pg)) }

const appHost = "vaxbook.test"

var testPepper = []byte("test-pepper-test-pepper-test-pepper!")

// harness drives the real root handler, wired as in main, against a fresh database.
type harness struct {
	t      *testing.T
	h      http.Handler
	mailer *fakeMailer
	clock  *clock.Fake
	logs   *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db := pg.NewDB(t)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(&lockedWriter{w: logs}, nil))
	clk := clock.NewFake(time.Date(2026, 11, 21, 9, 0, 0, 123456789, time.UTC)) // sub-µs part catches precision bugs
	mailer := &fakeMailer{codes: map[string]string{}}
	h, err := httpapi.NewHandler(httpapi.Options{
		AppHost:  appHost,
		Identity: identity.New(store.NewClient(db), mailer, clk, testPepper),
		Ping:     db.PingContext,
		Logger:   logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &harness{t: t, h: h, mailer: mailer, clock: clk, logs: logs}
}

// req describes one request. Write requests carry the CSRF header unless noCSRF.
type req struct {
	method string
	path   string
	body   string
	host   string
	cookie string
	noCSRF bool
}

func (h *harness) do(r req) *httptest.ResponseRecorder {
	h.t.Helper()
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	hr := httptest.NewRequest(r.method, r.path, body)
	hr.Host = appHost
	if r.host != "" {
		hr.Host = r.host
	}
	if r.body != "" {
		hr.Header.Set("Content-Type", "application/json")
	}
	if r.method != http.MethodGet && !r.noCSRF {
		hr.Header.Set("X-VaxBook-CSRF", "1")
	}
	if r.cookie != "" {
		hr.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: r.cookie})
	}
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, hr)
	return rec
}

func (h *harness) requestOTP(email string) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do(req{method: http.MethodPost, path: "/api/v1/auth/otp", body: jsonBody(map[string]string{"email": email})})
}

func (h *harness) verify(email, code string) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do(req{method: http.MethodPost, path: "/api/v1/auth/otp/verify", body: jsonBody(map[string]string{"email": email, "code": code})})
}

// signIn runs the whole OTP flow and returns the session token and account.
func (h *harness) signIn(email string) (string, me) {
	h.t.Helper()
	if rec := h.requestOTP(email); rec.Code != http.StatusAccepted {
		h.t.Fatalf("request otp: %d %s", rec.Code, rec.Body)
	}
	rec := h.verify(email, h.mailer.code(h.t, normalize(email)))
	if rec.Code != http.StatusOK {
		h.t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	c := sessionCookie(h.t, rec)
	return c.Value, decode[me](h.t, rec)
}

type me struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type problemBody struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func requireProblem(t *testing.T, rec *httptest.ResponseRecorder, status int) problemBody {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	p := decode[problemBody](t, rec)
	if p.Status != status {
		t.Fatalf("problem status = %d, want %d", p.Status, status)
	}
	return p
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == httpapi.SessionCookie {
			return c
		}
	}
	t.Fatalf("no %s cookie in response; headers %v", httpapi.SessionCookie, rec.Header())
	return nil
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return v
}

func jsonBody(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func normalize(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// fakeMailer records the last code sent to each email.
type fakeMailer struct {
	mu    sync.Mutex
	codes map[string]string
	sent  int
	fail  bool
}

func (m *fakeMailer) SendOTP(_ context.Context, email, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("smtp down")
	}
	m.codes[email] = code
	m.sent++
	return nil
}

func (m *fakeMailer) code(t *testing.T, email string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.codes[email]
	if !ok {
		t.Fatalf("no code sent to %s", email)
	}
	return c
}

// wrongCode returns a well-formed code different from code.
func wrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

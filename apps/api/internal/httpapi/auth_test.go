package httpapi_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fugo101/vaxbook/apps/api/internal/httpapi"
)

func TestRequestOTPAnswersTheSameForKnownAndUnknownEmails(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.signIn("known@example.com")
	h.clock.Advance(time.Minute)

	known := h.requestOTP("known@example.com")
	unknown := h.requestOTP("unknown@example.com")

	for name, rec := range map[string]int{"known": known.Code, "unknown": unknown.Code} {
		if rec != http.StatusAccepted {
			t.Errorf("%s email: status %d, want 202", name, rec)
		}
	}
	if known.Body.String() != unknown.Body.String() {
		t.Errorf("bodies differ: %q vs %q", known.Body, unknown.Body)
	}
}

func TestRequestOTPRejectsInvalidEmail(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, email := range []string{"not-an-email", "Name <a@example.com>", ""} {
		t.Run(email, func(t *testing.T) {
			requireProblem(t, h.requestOTP(email), http.StatusBadRequest)
		})
	}
	if h.mailer.sent != 0 {
		t.Fatalf("sent %d codes, want 0", h.mailer.sent)
	}
}

func TestRequestOTPResendWaitsSixtySeconds(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if rec := h.requestOTP("a@example.com"); rec.Code != http.StatusAccepted {
		t.Fatalf("first: %d", rec.Code)
	}

	h.clock.Advance(59 * time.Second)
	rec := h.requestOTP("A@Example.com ") // same email after normalisation
	requireProblem(t, rec, http.StatusTooManyRequests)
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1", got)
	}

	h.clock.Advance(time.Second)
	if rec := h.requestOTP("a@example.com"); rec.Code != http.StatusAccepted {
		t.Fatalf("after 60s: %d %s", rec.Code, rec.Body)
	}
}

func TestVerifyCreatesAccountAndSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("  New.User@Example.COM ")

	rec := h.verify("new.user@example.com", h.mailer.code(t, "new.user@example.com"))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	c := sessionCookie(t, rec)
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" {
		t.Errorf("cookie attributes wrong: %+v", c)
	}
	if c.MaxAge != int((30 * 24 * time.Hour).Seconds()) {
		t.Errorf("cookie Max-Age = %d, want 30 days", c.MaxAge)
	}
	got := decode[me](t, rec)
	if got.Email != "new.user@example.com" || got.ID == "" {
		t.Errorf("me = %+v", got)
	}

	meRec := h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: c.Value})
	if meRec.Code != http.StatusOK {
		t.Fatalf("/me: %d %s", meRec.Code, meRec.Body)
	}
	if again := decode[me](t, meRec); again != got {
		t.Errorf("/me = %+v, want %+v", again, got)
	}
}

func TestSigningInTwiceReusesTheAccount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_, first := h.signIn("same@example.com")
	h.clock.Advance(time.Minute)
	_, second := h.signIn("SAME@example.com")
	if first.ID != second.ID {
		t.Fatalf("account ids differ: %s vs %s", first.ID, second.ID)
	}
}

func TestVerifyRejectsWrongCode(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	code := h.mailer.code(t, "a@example.com")

	p := requireProblem(t, h.verify("a@example.com", wrongCode(code)), http.StatusBadRequest)
	if strings.Contains(p.Detail, "attempt") {
		t.Errorf("detail leaks attempt counting: %q", p.Detail)
	}
	// A wrong guess does not burn the challenge.
	if rec := h.verify("a@example.com", code); rec.Code != http.StatusOK {
		t.Fatalf("right code after one wrong: %d %s", rec.Code, rec.Body)
	}
}

func TestFourWrongAttemptsStillAllowTheRightCode(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	code := h.mailer.code(t, "a@example.com")
	for range 4 {
		requireProblem(t, h.verify("a@example.com", wrongCode(code)), http.StatusBadRequest)
	}
	if rec := h.verify("a@example.com", code); rec.Code != http.StatusOK {
		t.Fatalf("right code after 4 wrong: %d %s", rec.Code, rec.Body)
	}
}

func TestFifthWrongAttemptKillsTheChallenge(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	code := h.mailer.code(t, "a@example.com")
	for range 5 {
		requireProblem(t, h.verify("a@example.com", wrongCode(code)), http.StatusBadRequest)
	}
	requireProblem(t, h.verify("a@example.com", code), http.StatusBadRequest)
}

// Regression: killing a challenge must not reset the resend wait, or a caller gets
// a fresh code and five fresh guesses with no wait at all.
func TestKilledChallengeStillEnforcesTheResendWait(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	code := h.mailer.code(t, "a@example.com")
	for range 5 {
		h.verify("a@example.com", wrongCode(code))
	}

	requireProblem(t, h.requestOTP("a@example.com"), http.StatusTooManyRequests)
	if h.mailer.sent != 1 {
		t.Fatalf("sent %d codes, want 1", h.mailer.sent)
	}

	h.clock.Advance(60 * time.Second)
	if rec := h.requestOTP("a@example.com"); rec.Code != http.StatusAccepted {
		t.Fatalf("after the wait: %d %s", rec.Code, rec.Body)
	}
	if rec := h.verify("a@example.com", h.mailer.code(t, "a@example.com")); rec.Code != http.StatusOK {
		t.Fatalf("new code after the wait: %d %s", rec.Code, rec.Body)
	}
}

func TestExpiredChallengeCanBeReplaced(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	h.clock.Advance(10 * time.Minute)
	requireProblem(t, h.verify("a@example.com", h.mailer.code(t, "a@example.com")), http.StatusBadRequest)
	if rec := h.requestOTP("a@example.com"); rec.Code != http.StatusAccepted {
		t.Fatalf("request after expiry: %d %s", rec.Code, rec.Body)
	}
}

func TestConcurrentWrongAttemptsCannotExceedTheLimit(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	code := h.mailer.code(t, "a@example.com")

	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() { h.verify("a@example.com", wrongCode(code)) })
	}
	wg.Wait()
	requireProblem(t, h.verify("a@example.com", code), http.StatusBadRequest)
}

func TestCodeExpiresAfterTenMinutes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	code := h.mailer.code(t, "a@example.com")
	h.clock.Advance(10 * time.Minute)
	requireProblem(t, h.verify("a@example.com", code), http.StatusBadRequest)
}

func TestNewCodeInvalidatesThePreviousOne(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.requestOTP("a@example.com")
	old := h.mailer.code(t, "a@example.com")
	h.clock.Advance(time.Minute)
	h.requestOTP("a@example.com")
	fresh := h.mailer.code(t, "a@example.com")
	if old == fresh {
		t.Skip("random codes collided; the property cannot be observed this run")
	}
	requireProblem(t, h.verify("a@example.com", old), http.StatusBadRequest)
	if rec := h.verify("a@example.com", fresh); rec.Code != http.StatusOK {
		t.Fatalf("fresh code: %d %s", rec.Code, rec.Body)
	}
}

func TestVerifyWithoutAPendingCode(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	requireProblem(t, h.verify("nobody@example.com", "123456"), http.StatusBadRequest)
}

func TestMailerFailureDoesNotLockTheEmailOut(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.mailer.fail = true
	requireProblem(t, h.requestOTP("a@example.com"), http.StatusInternalServerError)
	h.mailer.fail = false
	if rec := h.requestOTP("a@example.com"); rec.Code != http.StatusAccepted {
		t.Fatalf("retry after mail failure: %d %s", rec.Code, rec.Body)
	}
}

func TestMeRequiresASession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	requireProblem(t, h.do(req{method: http.MethodGet, path: "/api/v1/me"}), http.StatusUnauthorized)

	rec := h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: "not-a-real-token"})
	requireProblem(t, rec, http.StatusUnauthorized)
	if c := sessionCookie(t, rec); c.MaxAge >= 0 {
		t.Errorf("stale cookie not cleared: %+v", c)
	}
}

func TestLogoutRevokesTheSessionImmediately(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	token, _ := h.signIn("a@example.com")

	rec := h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", cookie: token})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if c := sessionCookie(t, rec); c.MaxAge >= 0 {
		t.Errorf("logout did not clear the cookie: %+v", c)
	}
	requireProblem(t, h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: token}), http.StatusUnauthorized)
}

func TestSessionIsExtendedByUseAndExpiresWhenIdle(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	token, _ := h.signIn("a@example.com")

	// Within the touch interval: no renewal, no cookie re-issued.
	h.clock.Advance(30 * time.Minute)
	rec := h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: token})
	if rec.Code != http.StatusOK || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("early use: %d, Set-Cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}

	// Day 20: renewed, cookie re-issued for another 30 days.
	h.clock.Advance(20*24*time.Hour - 30*time.Minute)
	rec = h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: token})
	if rec.Code != http.StatusOK {
		t.Fatalf("day 20: %d", rec.Code)
	}
	if c := sessionCookie(t, rec); c.Value != token || c.MaxAge != int((30*24*time.Hour).Seconds()) {
		t.Errorf("renewed cookie = %+v", c)
	}

	// Day 45 is past the original 30 days but within 30 days of day 20.
	h.clock.Advance(25 * 24 * time.Hour)
	if rec := h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: token}); rec.Code != http.StatusOK {
		t.Fatalf("day 45: %d", rec.Code)
	}

	// 30 idle days later the session is gone.
	h.clock.Advance(30 * 24 * time.Hour)
	requireProblem(t, h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: token}), http.StatusUnauthorized)
}

func TestWritesRequireTheCSRFHeader(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	token, _ := h.signIn("a@example.com")

	requireProblem(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/otp", body: `{"email":"b@example.com"}`, noCSRF: true}), http.StatusForbidden)
	requireProblem(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", cookie: token, noCSRF: true}), http.StatusForbidden)
	// The session survived the rejected logout.
	if rec := h.do(req{method: http.MethodGet, path: "/api/v1/me", cookie: token}); rec.Code != http.StatusOK {
		t.Fatalf("/me after rejected logout: %d", rec.Code)
	}
}

func TestRequestsAreValidatedAgainstTheSpec(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	cases := map[string]string{
		"malformed json":    `{"email":`,
		"unknown field":     `{"email":"a@example.com","admin":true}`,
		"missing field":     `{}`,
		"wrong type":        `{"email":42}`,
		"code not 6 digits": `{"email":"a@example.com","code":"12ab56"}`,
	}
	for name, body := range cases {
		path := "/api/v1/auth/otp"
		if strings.Contains(name, "code") {
			path = "/api/v1/auth/otp/verify"
		}
		t.Run(name, func(t *testing.T) {
			requireProblem(t, h.do(req{method: http.MethodPost, path: path, body: body}), http.StatusBadRequest)
		})
	}
	if h.mailer.sent != 0 {
		t.Fatalf("invalid requests sent %d codes", h.mailer.sent)
	}
}

func TestLargeBodiesAreRejected(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	body := `{"email":"` + strings.Repeat("a", httpapi.MaxBodyBytes) + `@example.com"}`
	requireProblem(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/otp", body: body}), http.StatusRequestEntityTooLarge)
}

func TestRoutingByHost(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	requireProblem(t, h.do(req{method: http.MethodGet, path: "/api/v1/me", host: "other.test"}), http.StatusNotFound)
	requireProblem(t, h.do(req{method: http.MethodGet, path: "/api/v1/nope"}), http.StatusNotFound)

	for _, host := range []string{appHost, "other.test", "127.0.0.1:8080"} {
		rec := h.do(req{method: http.MethodGet, path: "/healthz", host: host})
		if rec.Code != http.StatusOK {
			t.Errorf("/healthz on %s: %d", host, rec.Code)
		}
	}
}

func TestResponsesCarryARequestID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := h.do(req{method: http.MethodGet, path: "/healthz"})
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("no X-Request-Id header")
	}
}

func TestLogsHoldNoEmailOrCode(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.signIn("secret.person@example.com")
	h.requestOTP("not-an-email")
	logs := h.logs.String()
	if strings.Contains(logs, "secret.person") {
		t.Errorf("logs contain the email:\n%s", logs)
	}
	if strings.Contains(logs, `"code"`) {
		t.Errorf("logs contain a code field:\n%s", logs)
	}
	if !strings.Contains(logs, `"route":"POST /api/v1/auth/otp"`) {
		t.Errorf("access log does not name the route:\n%s", logs)
	}
}

func TestHostMatchingIgnoresCaseAndTrailingDot(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	for _, host := range []string{"VAXBOOK.TEST", "vaxbook.test.", "Vaxbook.Test:8443"} {
		requireProblem(t, h.do(req{method: http.MethodGet, path: "/api/v1/me", host: host}), http.StatusUnauthorized)
	}
}

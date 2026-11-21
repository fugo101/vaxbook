// Package identity handles email OTP sign-in and server-side sessions
// (F01.1, ADR-0010, SAD §9).
//
// Sign-up and sign-in are one flow: an Account is created the first time a code
// for its email is verified. Codes and emails are stored only as keyed HMACs;
// sessions only as a hash of the cookie token.
package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fugo101/vaxbook/apps/api/internal/platform/clock"
	"github.com/fugo101/vaxbook/apps/api/internal/store/ent"
	"github.com/fugo101/vaxbook/apps/api/internal/store/ent/account"
	"github.com/fugo101/vaxbook/apps/api/internal/store/ent/otpchallenge"
	"github.com/fugo101/vaxbook/apps/api/internal/store/ent/session"
)

// Limits from F01.1 / SAD §9.
const (
	CodeTTL          = 10 * time.Minute
	ResendInterval   = 60 * time.Second
	MaxWrongAttempts = 5
	SessionIdleTTL   = 30 * 24 * time.Hour
	// SessionTouchInterval bounds how often a session's expiry is pushed back, so
	// that reading a page does not write to the database on every request.
	SessionTouchInterval = time.Hour
)

var (
	// ErrInvalidEmail means the address is not a usable email address.
	ErrInvalidEmail = errors.New("identity: invalid email")
	// ErrInvalidCode covers a wrong, expired, exhausted or unknown code. The cases
	// are deliberately indistinguishable to the caller.
	ErrInvalidCode = errors.New("identity: invalid or expired code")
	// ErrNoSession means the token does not name a live session.
	ErrNoSession = errors.New("identity: no session")
)

// ResendTooSoonError means a code was sent to this email less than
// ResendInterval ago.
type ResendTooSoonError struct{ RetryAfter time.Duration }

func (e *ResendTooSoonError) Error() string {
	return fmt.Sprintf("identity: resend allowed in %s", e.RetryAfter)
}

// Account is the signed-in identity.
type Account struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
}

// Mailer delivers sign-in codes.
type Mailer interface {
	SendOTP(ctx context.Context, email, code string) error
}

// Service is the identity module.
type Service struct {
	db     *ent.Client
	mailer Mailer
	clock  clock.Clock
	pepper []byte
}

// New returns a Service. pepper is the HMAC key for stored emails and codes.
func New(db *ent.Client, mailer Mailer, clk clock.Clock, pepper []byte) *Service {
	return &Service{db: db, mailer: mailer, clock: clk, pepper: pepper}
}

// NormalizeEmail trims and lowercases an address and checks that it is a bare
// email address. It does not apply provider-specific rules (such as Gmail dots).
func NormalizeEmail(raw string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" || len(e) > 254 {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e || addr.Name != "" {
		return "", ErrInvalidEmail
	}
	return e, nil
}

// RequestOTP sends a fresh sign-in code to email, replacing any pending code.
// It behaves the same whether or not the email has an account.
func (s *Service) RequestOTP(ctx context.Context, rawEmail string) error {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	code, err := newCode()
	if err != nil {
		return err
	}
	emailHash := s.mac("email", email)
	now := s.now()

	var challengeID uuid.UUID
	err = s.withTx(ctx, func(tx *ent.Tx) error {
		existing, err := tx.OtpChallenge.Query().
			Where(otpchallenge.EmailHash(emailHash)).
			ForUpdate().
			Only(ctx)
		switch {
		case ent.IsNotFound(err):
		case err != nil:
			return err
		case now.Before(existing.ResendAfter):
			return &ResendTooSoonError{RetryAfter: existing.ResendAfter.Sub(now)}
		default:
			if err := tx.OtpChallenge.DeleteOne(existing).Exec(ctx); err != nil {
				return err
			}
		}
		c, err := tx.OtpChallenge.Create().
			SetEmailHash(emailHash).
			SetCodeHash(s.mac("code", email+"\x00"+code)).
			SetCreatedAt(now).
			SetExpiresAt(now.Add(CodeTTL)).
			SetResendAfter(now.Add(ResendInterval)).
			Save(ctx)
		if err != nil {
			return err
		}
		challengeID = c.ID
		return nil
	})
	if ent.IsConstraintError(err) {
		// A concurrent request for the same email inserted its challenge first.
		return &ResendTooSoonError{RetryAfter: ResendInterval}
	}
	if err != nil {
		return err
	}

	if err := s.mailer.SendOTP(ctx, email, code); err != nil {
		// Do not leave the user locked out for ResendInterval by a code they never got.
		_, _ = s.db.OtpChallenge.Delete().Where(otpchallenge.ID(challengeID)).Exec(context.WithoutCancel(ctx))
		return fmt.Errorf("identity: send code: %w", err)
	}
	return nil
}

// VerifyOTP checks a code. On success it consumes the challenge, creates the
// account if this is the email's first sign-in, and opens a session. It returns
// the account and the session token to put in the cookie.
func (s *Service) VerifyOTP(ctx context.Context, rawEmail, code string) (Account, string, error) {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return Account{}, "", ErrInvalidCode
	}
	emailHash := s.mac("email", email)
	codeHash := s.mac("code", email+"\x00"+code)
	now := s.now()

	var (
		acc   *ent.Account
		token string
		wrong bool
	)
	err = s.withTx(ctx, func(tx *ent.Tx) error {
		// The row lock serialises concurrent verifications of the same email, so the
		// attempt counter cannot be raced and the account is created only once.
		c, err := tx.OtpChallenge.Query().
			Where(otpchallenge.EmailHash(emailHash)).
			ForUpdate().
			Only(ctx)
		if ent.IsNotFound(err) {
			wrong = true
			return nil
		}
		if err != nil {
			return err
		}
		if !now.Before(c.ExpiresAt) || c.Attempts >= MaxWrongAttempts {
			wrong = true
			return tx.OtpChallenge.DeleteOne(c).Exec(ctx)
		}
		if !hmac.Equal(c.CodeHash, codeHash) {
			wrong = true
			if c.Attempts+1 >= MaxWrongAttempts {
				return tx.OtpChallenge.DeleteOne(c).Exec(ctx)
			}
			return tx.OtpChallenge.UpdateOne(c).AddAttempts(1).Exec(ctx)
		}

		if err := tx.OtpChallenge.DeleteOne(c).Exec(ctx); err != nil {
			return err
		}
		acc, err = tx.Account.Query().Where(account.Email(email)).Only(ctx)
		if ent.IsNotFound(err) {
			acc, err = tx.Account.Create().SetEmail(email).SetCreatedAt(now).Save(ctx)
		}
		if err != nil {
			return err
		}
		token, err = s.createSession(ctx, tx, acc.ID, now)
		return err
	})
	if err != nil {
		return Account{}, "", err
	}
	if wrong {
		return Account{}, "", ErrInvalidCode
	}
	return toAccount(acc), token, nil
}

// Session is an authenticated session.
type Session struct {
	Account   Account
	ExpiresAt time.Time
	// Renewed reports that ExpiresAt was just pushed back, so the caller should
	// re-issue the cookie with the new expiry.
	Renewed bool
}

// Authenticate resolves a session token to its account and pushes back the
// session's expiry (at most once per SessionTouchInterval).
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	id, ok := sessionID(token)
	if !ok {
		return Session{}, ErrNoSession
	}
	now := s.now()
	sess, err := s.db.Session.Query().
		Where(session.ID(id), session.ExpiresAtGT(now)).
		WithAccount().
		Only(ctx)
	if ent.IsNotFound(err) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, err
	}
	out := Session{Account: toAccount(sess.Edges.Account), ExpiresAt: sess.ExpiresAt}
	if now.Sub(sess.LastSeenAt) >= SessionTouchInterval {
		out.ExpiresAt = now.Add(SessionIdleTTL)
		n, err := s.db.Session.Update().
			Where(session.ID(id)).
			SetLastSeenAt(now).
			SetExpiresAt(out.ExpiresAt).
			Save(ctx)
		if err != nil {
			return Session{}, err
		}
		if n == 0 { // logged out concurrently
			return Session{}, ErrNoSession
		}
		out.Renewed = true
	}
	return out, nil
}

// Logout ends the session named by token. Unknown tokens are not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	id, ok := sessionID(token)
	if !ok {
		return nil
	}
	_, err := s.db.Session.Delete().Where(session.ID(id)).Exec(ctx)
	return err
}

func (s *Service) createSession(ctx context.Context, tx *ent.Tx, accountID uuid.UUID, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	id, _ := sessionID(token)
	err := tx.Session.Create().
		SetID(id).
		SetAccountID(accountID).
		SetCreatedAt(now).
		SetLastSeenAt(now).
		SetExpiresAt(now.Add(SessionIdleTTL)).
		Exec(ctx)
	return token, err
}

// sessionID maps a cookie token to its stored id: hex(SHA-256(token)). Tokens
// that are not 32 random bytes in base64url are rejected without a query.
func sessionID(token string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return "", false
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), true
}

// mac is HMAC-SHA256 keyed by the server pepper, with a purpose label so an email
// hash can never be confused with a code hash.
func (s *Service) mac(purpose, value string) []byte {
	h := hmac.New(sha256.New, s.pepper)
	h.Write([]byte(purpose))
	h.Write([]byte{0})
	h.Write([]byte(value))
	return h.Sum(nil)
}

// newCode returns a uniformly random 6-digit code.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// now is the current time at Postgres precision (microseconds), so a value
// returned right after a write equals the value read back later.
func (s *Service) now() time.Time {
	return s.clock.Now().UTC().Truncate(time.Microsecond)
}

func (s *Service) withTx(ctx context.Context, fn func(*ent.Tx) error) error {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func toAccount(a *ent.Account) Account {
	return Account{ID: a.ID, Email: a.Email, CreatedAt: a.CreatedAt}
}

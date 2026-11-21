package identity

import (
	"context"
	"log/slog"
)

// ConsoleMailer writes sign-in codes to the log instead of sending email. It
// exists for local development only; config refuses it outside VAXBOOK_ENV=dev
// because it puts emails and codes in the log.
type ConsoleMailer struct{ Logger *slog.Logger }

// SendOTP logs the code.
func (m ConsoleMailer) SendOTP(ctx context.Context, email, code string) error {
	m.Logger.WarnContext(ctx, "DEV ONLY: sign-in code (console mailer)",
		slog.String("email", email), slog.String("code", code))
	return nil
}

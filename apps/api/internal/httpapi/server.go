// Package httpapi is the HTTP layer of the user API (`/api/v1`). It implements the
// interface generated from contracts/openapi.yaml and only translates between HTTP
// and the domain modules; business rules live in the modules.
package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/fugo101/vaxbook/apps/api/internal/identity"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/problem"
)

// SessionCookie is the session cookie name. The __Host- prefix forces Secure,
// Path=/ and no Domain, so sibling apps on the same parent domain cannot set or
// overwrite it (ADR-0011).
const SessionCookie = "__Host-session"

// server implements StrictServerInterface.
type server struct {
	identity *identity.Service
}

var _ StrictServerInterface = (*server)(nil)

func (s *server) RequestOtp(ctx context.Context, req RequestOtpRequestObject) (RequestOtpResponseObject, error) {
	err := s.identity.RequestOTP(ctx, req.Body.Email)
	var tooSoon *identity.ResendTooSoonError
	switch {
	case err == nil:
		return RequestOtp202Response{}, nil
	case errors.Is(err, identity.ErrInvalidEmail):
		return RequestOtp400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: BadRequestApplicationProblemPlusJSONResponse(toProblem(http.StatusBadRequest, "invalid email address")),
		}, nil
	case errors.As(err, &tooSoon):
		secs := int(math.Ceil(tooSoon.RetryAfter.Seconds()))
		return RequestOtp429ApplicationProblemPlusJSONResponse{
			TooManyRequestsApplicationProblemPlusJSONResponse: TooManyRequestsApplicationProblemPlusJSONResponse{
				Body:    toProblem(http.StatusTooManyRequests, "a code was sent recently; wait before asking for another"),
				Headers: TooManyRequestsResponseHeaders{RetryAfter: &secs},
			},
		}, nil
	default:
		return nil, err
	}
}

func (s *server) VerifyOtp(ctx context.Context, req VerifyOtpRequestObject) (VerifyOtpResponseObject, error) {
	acc, token, err := s.identity.VerifyOTP(ctx, req.Body.Email, req.Body.Code)
	if errors.Is(err, identity.ErrInvalidCode) {
		return VerifyOtp400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: BadRequestApplicationProblemPlusJSONResponse(toProblem(http.StatusBadRequest, "the code is wrong or has expired")),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	cookie := sessionCookie(token, identity.SessionIdleTTL)
	return VerifyOtp200JSONResponse{
		Body:    toMe(acc),
		Headers: VerifyOtp200ResponseHeaders{SetCookie: &cookie},
	}, nil
}

func (s *server) Logout(ctx context.Context, _ LogoutRequestObject) (LogoutResponseObject, error) {
	a := authFrom(ctx)
	if err := s.identity.Logout(ctx, a.token); err != nil {
		return nil, err
	}
	cleared := sessionCookie("", -1)
	return Logout204Response{Headers: Logout204ResponseHeaders{SetCookie: &cleared}}, nil
}

func (s *server) GetMe(ctx context.Context, _ GetMeRequestObject) (GetMeResponseObject, error) {
	return GetMe200JSONResponse(toMe(authFrom(ctx).session.Account)), nil
}

// sessionCookie renders the Set-Cookie value. A negative maxAge deletes the cookie.
func sessionCookie(token string, maxAge time.Duration) string {
	c := &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if maxAge < 0 {
		c.MaxAge = -1
	} else {
		c.MaxAge = int(maxAge.Seconds())
	}
	return c.String()
}

func toMe(a identity.Account) Me {
	return Me{Id: a.ID, Email: openapi_types.Email(a.Email), CreatedAt: a.CreatedAt.UTC()}
}

func toProblem(status int, detail string) Problem {
	p := problem.New(status, detail)
	return Problem{Type: p.Type, Title: p.Title, Status: p.Status, Detail: &p.Detail}
}

# Server-side sessions, no JWT for users

Email OTP login creates a session stored in Postgres. The cookie is `__Host-session` with the flags `HttpOnly`, `Secure`, `SameSite=Lax`; the session expires after 30 days of inactivity and is extended on each use.

Reason: the spec requires access to be cut **immediately** on logout (F01.1), account deletion or consent withdrawal (F01.4, F01.5), and email change (F01.6). A JWT cannot be revoked before it expires unless a revoked-token list is added, and that list is in effect a session table. The "stateless" advantage of JWT has no value when there is only one binary, and almost every request already reads Postgres anyway.

## Considered Options

- JWT stored in localStorage: a single XSS and the token is compromised. Not acceptable for health data.
- Short-lived access JWT plus a refresh token stored in the DB: in effect still a session, just with more complexity.

JWT is still used where it fits: verifying the JWT that Cloudflare Access issues for the admin (ADR-0008).

# Admin authentication is delegated to Cloudflare Access; MCP uses two token layers

The admin page (`vaxbook-admin.<operator-domain>`) is protected by Cloudflare Access: OTP sent to the exact admin email, Free plan. The application verifies the `Cf-Access-Jwt-Assertion` JWT itself, and only accepts the configured admin email. VaxBook therefore has no admin login system of its own.

The MCP endpoint (`vaxbook-mcp.<operator-domain>`) requires two layers: a **Cloudflare Access service token** (does not count toward the plan's user count) and an **application bearer token** (stored as a hash, rotatable).

## Consequences

- No official documentation confirms yet that the Free plan includes service tokens. This must be tested directly on the dashboard. If they are not available, MCP uses only the application bearer token, plus rate limiting.
- Cloudflare's free SSL certificate (Universal SSL) only covers one level of subdomain, so use `vaxbook-admin` instead of `admin.vaxbook`.

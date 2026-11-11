# Hosting on a VPS in Vietnam; Cloudflare and Resend accepted as cross-border data processors

VaxBook runs with docker compose on a VPS in Vietnam, behind Traefik and Cloudflare Tunnel, following the operator's existing conventions. The database and all stored data stay in the country. Two data flows go abroad, and both are accepted on purpose:

- **Cloudflare** terminates TLS at the edge, so requests and responses (including vaccination data) are in plaintext at the data center serving the user. Restricting the processing region is only available on the Enterprise plan.
- **Resend** stores email content and metadata in the US for 30 days, regardless of the sending region. Reminder emails **keep their full content per F08** (nickname plus disease name), so health data does pass through Resend. The spec already guarantees that emails never mention pregnancy.

Reasons: the operator's existing setup, low budget, and email deliverability. Both parties will be declared in the cross-border data transfer impact assessment dossier under Luật 91/2025 (Law 91/2025 on Personal Data Protection). A lawyer must review this before public launch (see ADR-0007).

## Considered Options

- Notification-only email ("Bạn có N lịch nhắc mới" ("You have N new reminders")): less data goes abroad, but this was rejected to keep emails useful as F08 describes.
- Open port 443 on the VPS and drop Cloudflare: contradicts the existing setup and loses the DDoS protection layer.

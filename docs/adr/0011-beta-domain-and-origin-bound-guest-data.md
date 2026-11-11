# Beta runs on vaxbook.<operator-domain>; changing the domain will make guests lose their data

The beta uses `vaxbook.<operator-domain>` (app), `vaxbook-admin.<operator-domain>` and `vaxbook-mcp.<operator-domain>`, through the existing `*.<operator-domain>` tunnel. Email is sent from the `vaxbook.<operator-domain>` subdomain so it does not affect the sending reputation of the main domain. All cookies use the `__Host-` prefix to prevent cookie tossing from other apps on the same `*.<operator-domain>`.

## Consequences

Guest data lives in IndexedDB, and IndexedDB is **bound to the domain (origin)**. When moving to a dedicated domain (a pre-launch condition, ADR-0007), guests will lose the data on their device. Before the move there must be a period that shows a banner reminding users to use "Tải dữ liệu của tôi" ("Download my data"), and the new domain must support re-importing the downloaded file.

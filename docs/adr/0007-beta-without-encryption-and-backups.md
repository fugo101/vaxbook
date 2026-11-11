# Beta phase: no encryption at rest and no backups

During the closed beta, the database is not yet encrypted at rest (the VPS disk is not encrypted either) and there are **no backups at all**, including for the rule set. This is a deliberate decision by the operator, to keep the beta simple. The "Về dữ liệu" ("About the data") page must state clearly that data may be lost if the server fails, and advise users to use "Tải dữ liệu của tôi" ("Download my data") periodically. A resulting consequence: deleted data is gone immediately and does not survive in any backup (F01.4).

## Consequences

This is a **blocking condition for public launch**. Before public launch, all of the following must be in place:
1. Encryption at rest (Nghị định 356/2025 (Decree 356/2025), Article 12(4), UNVERIFIED against the Công báo (Official Gazette)).
2. Off-machine backups, encrypted, with the retention period stated on the "Về dữ liệu" ("About the data") page.
3. A lawyer's opinion on the impact assessment dossier and the cross-border data transfer dossier (ADR-0005).
4. A doctor or pharmacist checks the rule set (spec §9).
5. A dedicated domain (ADR-0011).

Rule set backup via git is a high-priority post-MVP feature.

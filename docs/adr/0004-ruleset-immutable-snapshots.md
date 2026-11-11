# Each rule set version is an immutable JSON snapshot; a changeset is operations on stable IDs

Each published `RuleSetVersion` is stored as an immutable JSON document (a `jsonb` column with a checksum). The document has two parts: the public part (catalog and rules, sent to the browser for the engine) and the admin-only part (test cases). A changeset's `ChangeOperation` is an add, edit or soft delete operation on the **stable, never-reused ID** of each entity. Other operations:
- **Publish:** apply the operations to the base snapshot, validate, then create a new snapshot.
- **Diff:** structural comparison between two snapshots.
- **Rollback:** copy the target snapshot into a new version.

The catalog (Disease, Product, Series…) therefore **has no separate relational tables**. Reasons: the catalog has only a few hundred entities; the engine needs exactly this document; and a versioned relational schema would be cumbersome and would also need a separate JSON export.

## Consequences

- Catalog content cannot be queried with SQL. Admin pages work on the snapshot loaded into memory.
- Snapshots have a `schemaVersion` field. When the snapshot structure changes, older versions must be converted on read.

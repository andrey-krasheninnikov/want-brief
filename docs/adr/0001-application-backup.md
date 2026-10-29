# Application backup format and restore policy

## Context

Users need to recover personal records and settings through the application. The database also stores sessions, password hashes, and sealed source tokens, which should not travel in a downloadable file.

## Decision

Export application data as versioned JSON with stable record IDs and all current data tables. Omit authentication records, migration history, and source tokens. Accept a backup only when its format and schema versions match the running application. Restore by replacing application tables within one PostgreSQL transaction after explicit confirmation in the interface.

## Consequences

Relationships survive restoration, and a failed restore rolls back. Repeated restoration does not create duplicates. Existing records are removed on a successful restore, so the interface warns users to download a current backup first. Source tokens must be entered again. Schema changes require a deliberate backup format update before older files can be restored.

[ADR 0002](0002-scheduled-source-sync.md) adds a checked conversion for schema version 47 backups.

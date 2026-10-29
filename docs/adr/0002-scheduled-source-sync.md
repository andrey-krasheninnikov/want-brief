# Scheduled source sync

## Context

Jira and Todoist sync requires a manual request. Users need a schedule for each connection and a way to distinguish failed attempts from successful imports. The application can run more than one process against the same PostgreSQL database.

## Decision

Keep the schedule and attempt status on each source. A process checks due sources once a minute. Manual and scheduled requests claim a source in PostgreSQL before contacting it. A claim expires after ten minutes, while a remote request has a five-minute limit. A second request cannot import the same source concurrently. The final write checks the claim and source configuration again. Failed imports update the source error without changing existing items.

Schema version 48 adds these fields. Exports omit active claim fields. The restore path accepts version 47 backups by filling the new source fields with manual mode and no prior attempt. Other versions remain unsupported.

## Consequences

Existing sources stay in manual mode until a schedule is selected. A failed source is retried after its interval if it still has a token. A process crash leaves a claim until it expires; the next due check can then retry. Scheduled imports are read-only for Jira and Todoist.

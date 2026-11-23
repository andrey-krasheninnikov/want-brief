# Want Brief

Want Brief keeps Jira, Todoist, and local work in one place. It combines task priorities, events, a weekly schedule, parallel time tracking, and daily check-ins.

## What you can do

- Review tasks in a board or an Eisenhower matrix, and keep project, person, and company records alongside them.
- Plan work around events and available time. The schedule shows capacity, at-risk items, and what-if comparisons.
- Run more than one timer at once. Pulse shows check-ins and time allocation; Today shows the current plan and earnings.
- Connect Jira and Todoist sources. Synchronization imports work and is read-only for those services.

## Local setup

You need Rust 1.85 or newer, Bun 1.3.14, Docker Compose, and an SMTP mailbox for registration. For local testing, run a development mailbox on `127.0.0.1:1025`. Its inbox lets you open confirmation, recovery, and invitation emails.

Copy the example settings and export them in the terminal that runs the API. The sample database credentials and token key are for local use.

```sh
cp .env.example .env
set -a
. ./.env
set +a
cd web && bun install --frozen-lockfile && cd ..
make up
make api
```

Run `make web` in another terminal and open <http://127.0.0.1:5173>. Choose **Create account**, enter your email, and open the confirmation link to set a password. A new account starts with a personal Free workspace. To use the scratch database and a separate web port, run `make api-dev` and `make web-dev`, then open <http://127.0.0.1:5174>.

### Email and sessions

Set `PUBLIC_APP_URL` to the browser-facing HTTPS origin, such as `https://brief.example.org`. The server uses it for email links and secure session cookies. If omitted, it uses `CORS_ORIGIN`. HTTP is supported on loopback for local development.

Configure `SMTP_HOST` and `SMTP_FROM`. The default transport is `SMTP_TLS=starttls` on port 587. Use `SMTP_TLS=tls` for implicit TLS on port 465. `SMTP_PORT` overrides the port. Set both `SMTP_USERNAME` and `SMTP_PASSWORD` when the relay requires authentication. A loopback test mailbox can use `SMTP_TLS=none` without credentials.

Registration and email binding links expire after 24 hours. Submit the registration form again to request another link. **Forgot password?** sends a 30-minute recovery link. A successful reset signs the account out on every device. The **Account** page lists active sessions and lets you revoke one.

## Existing installations

Rehearse the upgrade on a restored PostgreSQL copy. Keep the existing `TOKEN_KEY` so saved source credentials remain readable. The migration account needs permission to create a database role and grant membership. Configure a test SMTP mailbox for the rehearsal.

1. Schedule a maintenance window. Stop every old API instance and source-sync worker before taking the final PostgreSQL backup. Record the previous application version and keep its configuration with the backup.
2. Point `DATABASE_URL` at the database to upgrade and run `cargo run -- -migrate-only`. The migration keeps the sole installation owner's ID and saved records, and gives that owner's personal workspace a permanent Paid plan. If the database contains records with no identifiable owner, resolve ownership on a copy before retrying.
3. Start the new API and web app. Choose **Use installation password** and sign in with the existing password. In **Account**, enter an email and the existing password. Open the emailed link, then sign in with that email and password. Confirmation closes previous sessions and disables password-only sign-in.
4. Check the owner's tasks, sources, and schedule, then verify access from a fresh Free account before reopening the installation. For rollback, stop the new API and workers, restore the pre-upgrade backup to a separate database, and start the previous version against it. Reconcile any changes made after the backup separately.

### Workspace plans and invitations

A Free workspace allows two external sources and a schedule for today. Paid adds unlimited external sources, shared editing, and a 28-day schedule. Plans apply to individual workspaces. The single-owner installation keeps its permanent personal Paid plan after migration.

To change a workspace plan, use its ID from the workspace URL or `GET /api/me` and run the operator command with the deployment's `DATABASE_URL`.

```sh
want-brief set-plan <workspace-id> paid
want-brief set-plan <workspace-id> free
```

Paid schedule analysis adjusts estimates from tracked time, checks energy and focus, and uses work history to identify productive hours. Free schedules keep ordinary planning and what-if comparisons.

The owner of a paid team workspace can send or revoke invitations in **Account**. Invitations expire after seven days. The recipient signs in with the invited email to join as an editor. A downgrade keeps memberships and records. Editors retain read access until the workspace returns to Paid.

## Sources and time

Add Jira or Todoist connections under Sources. Jira uses a site URL and either a bearer token or an email and API token; Todoist uses a REST token. The default Jira query selects assigned, unfinished issues.

New workspaces connect to Jira Cloud sites on `https://<site>.atlassian.net` and to `https://api.todoist.com`. For a self-hosted Jira server, set `SOURCE_ALLOWED_ORIGINS` on the API process to a comma-separated list such as `https://jira.example.org,https://projects.example.org:8443`. Each entry grants access to that scheme, host and port. A source can include a deployment path such as `/jira`. New workspaces require certificate verification for HTTPS connections. The personal workspace migrated from a single-owner installation retains access to its operator's HTTP and HTTPS servers.

Each source starts in manual sync mode. Select a 15-minute, hourly, six-hour, or daily interval to sync it automatically. The first attempt starts within a minute; later attempts follow the selected interval from the previous attempt. Sources shows the last attempt, last successful sync, and any error. Failed attempts leave imported items in place and are retried on schedule while the source has a token. Manual sync remains available.

Each timer creates a time interval for an item. **Allocated time** sums intervals, including overlaps. **Wall-clock time** counts overlapping intervals only once.

## Application backups

The workspace owner can download and restore a JSON copy from **Sources**. A personal backup contains the owner's records and settings. A team backup contains shared records. It excludes each member's private notes, time entries, check-ins, personal settings and financial details. Both kinds omit account records, memberships, subscription plans, sessions, invitation links and source tokens. Source configuration, including email addresses and URLs, is stored in plain text. Keep the file private.

Choose a file, review its date and record count, then confirm replacement. Version 52 backups can be restored to a workspace of the same kind. Personal workspaces also accept version 47 and 48 files. Version 47 sources use manual sync after restore. The limit is 64 MiB. Restore keeps record IDs and relationships and commits in one transaction. A team restore that would remove a member's private records is rejected. Re-enter source tokens to reconnect. Use a PostgreSQL backup for an installation upgrade or rollback.

## Checks and documentation

Run `make test` for Rust tests and TypeScript checks. Run `make lint`, `cd web && bun test`, `bun run lint`, and `bun run build` for the remaining local checks. To run database integration tests, set `TEST_DATABASE_URL` to an isolated database whose name starts with `wantbrief_issue92_`, then run `cargo test --tests -- --ignored --test-threads=1`.

See [architecture](docs/architecture.md) for the application layout, [glossary](GLOSSARY.md) for domain terms, and [frontend notes](web/README.md) for web commands. Repository workflow conventions live in `docs/agents/`.

The [Want Brief design specification](docs/design/redesign.md) includes a responsive preview. The web interface uses Want Desyst v0.3.0 components and tokens.

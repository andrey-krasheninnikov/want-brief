# Want Brief

Want Brief keeps Jira, Todoist, and local work in one place. It combines task priorities, events, a weekly schedule, parallel time tracking, and daily check-ins.

## What you can do

- Review tasks in a board or an Eisenhower matrix, and keep project, person, and company records alongside them.
- Plan work around events and available time. The schedule shows capacity, at-risk items, and what-if comparisons.
- Run more than one timer at once. Pulse shows check-ins and time allocation; Today shows the current plan and earnings.
- Connect Jira and Todoist sources. Synchronization imports work and is read-only for those services.

## Local setup

You need Go 1.25.1, Bun 1.3.14, and Docker Compose. Install the web dependencies once:

```sh
cd web && bun install --frozen-lockfile && cd ..
```

Start PostgreSQL and the API from the repository root:

```sh
make up
make api
```

In another terminal, start the web app:

```sh
make web
```

Open <http://127.0.0.1:5173>. Sign in with `BOOTSTRAP_PASSWORD`; its local default is defined in the Makefile.

The Makefile provides development defaults. To use values from a local `.env` file, copy `.env.example`, set your own values, and export them in each terminal before running `make`:

```sh
cp .env.example .env
set -a
. ./.env
set +a
```

The example password, token key, database credentials, and HTTP settings are for local use. Do not expose this setup to an untrusted network.

For a separate scratch database and web port, run `make api-dev` and `make web-dev` in two additional terminals. The scratch web app opens at <http://127.0.0.1:5174>.

## Sources and time

Add Jira or Todoist connections under Sources, then start a sync there. Jira uses a site URL and either a bearer token or an email and API token; Todoist uses a REST token. The default Jira query selects assigned, unfinished issues.

Each timer creates a time interval for an item. **Allocated time** sums intervals, including overlaps. **Wall-clock time** counts overlapping intervals only once.

## Checks and documentation

Run `make test` for Go tests and TypeScript checks. Run `go vet ./...`, `cd web && bun test`, `bun run lint`, and `bun run build` for the remaining local checks. `make lint` also rewrites Go formatting; use `gofmt -l` when you only need to inspect it.

See [architecture](docs/architecture.md) for the application layout, [glossary](GLOSSARY.md) for domain terms, and [frontend notes](web/README.md) for web commands. Repository workflow conventions live in `docs/agents/`.

The [Want Brief design specification](docs/design/redesign.md) includes a responsive preview for the planned interface.

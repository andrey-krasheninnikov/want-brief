# Architecture

Want Brief is a React web app backed by a Rust HTTP API and PostgreSQL. The Vite development server proxies API requests to the Rust process. The process runs SQL migrations at startup, opens the database, and serves the API.

## Backend

- `src/main.rs` reads configuration, runs migrations, and starts the server.
- `src/http.rs` defines routes, session checks, and shared HTTP responses. Domain routes live in adjacent modules for items, events, people, companies, tracking, and sources.
- `src/schedule_plan.rs` calculates work and follow-up schedules. `src/schedule.rs` loads its inputs and serves schedule requests.
- `src/migrate.rs` applies the existing PostgreSQL migrations. `src/crypto.rs` seals source credentials; `src/auth.rs` manages sessions.
- `src/backup.rs` exports application tables from a consistent snapshot and replaces their contents in one transaction. It omits authentication records and source tokens.
- `src/sources.rs` handles manual and scheduled source sync. A background loop started by `src/main.rs` checks due sources each minute; PostgreSQL claims keep concurrent runs from importing the same source at once.

Jira and Todoist are import sources. Sync reads remote work into local items; local edits and scheduling do not write back to those services. A manual source holds locally created items. Source credentials are sealed before storage and are not returned by the public source representation.

## Web app

`web/src/App.tsx` declares routes and the sign-in gate. Screens under `web/src/features/` cover Today, Tasks, Events, Schedule, Priorities, Projects, People, Companies, Pulse, and Sources. `web/src/api.ts` is the typed API client. Shared controls and layout live under `web/src/shared/`.

## Planning and time

The schedule combines items, events, tracked time, check-ins, and schedule settings. Its settings include a time zone, work window, capacity, and packing preferences. What-if comparisons evaluate alternatives without changing saved work.

Time intervals belong to items and may overlap. Allocated time sums their durations; wall-clock time merges overlap. See [GLOSSARY.md](../GLOSSARY.md) for these and other domain terms.

## Local environment

The Makefile has separate live and scratch database and port targets. Its default credentials and the values in `.env.example` are development examples. Supply appropriate secrets and network settings before using the app outside a trusted local environment.

## Refactoring

The [staged architecture review](architecture/refactoring-93.html) evaluates Source and Item sync, Schedule views, and the schedule planner. It links the recommended follow-up issues.

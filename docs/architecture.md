# Architecture

Want Brief is a React web app backed by a Go HTTP API and PostgreSQL. The Vite development server proxies API requests to the Go process. The Go process runs SQL migrations at startup, opens the database, and serves the API.

## Backend

- `internal/domain` defines items, events, people, companies, time intervals, scheduling rules, and repository contracts.
- `internal/application` coordinates use cases such as source sync, item updates, time tracking, and schedule calculation.
- `internal/infrastructure` contains PostgreSQL repositories, Jira and Todoist pullers, and token encryption.
- `internal/delivery/http` exposes the API. `internal/bootstrap` wires dependencies and starts the server.

Jira and Todoist are import sources. Sync reads remote work into local items; local edits and scheduling do not write back to those services. A manual source holds locally created items. Source credentials are sealed before storage and are not returned by the public source representation.

## Web app

`web/src/App.tsx` declares routes and the sign-in gate. Screens under `web/src/features/` cover Today, Tasks, Events, Schedule, Priorities, Projects, People, Companies, Pulse, and Sources. `web/src/api.ts` is the typed API client. Shared controls and layout live under `web/src/shared/`.

## Planning and time

The schedule combines items, events, tracked time, check-ins, and schedule settings. Its settings include a time zone, work window, capacity, and packing preferences. What-if comparisons evaluate alternatives without changing saved work.

Time intervals belong to items and may overlap. Allocated time sums their durations; wall-clock time merges overlap. See [GLOSSARY.md](../GLOSSARY.md) for these and other domain terms.

## Local environment

The Makefile has separate live and scratch database and port targets. Its default credentials and the values in `.env.example` are development examples. Supply appropriate secrets and network settings before using the app outside a trusted local environment.

# Workspace ownership and private records

## Context

A single-owner installation can contain years of work, active sessions and encrypted source credentials. After registration opens, each new account needs its own records. Members of the same team also need to keep personal data private from one another.

## Decision

Migrations 49–52 assign existing data to the sole installation owner’s personal workspace and retain its permanent Paid plan. Accounts created through email confirmation start with a personal Free workspace. A migration with ambiguous ownership stops before assigning the affected records. Operators rehearse the upgrade on a copy and keep a PostgreSQL backup for rollback.

The HTTP API authenticates the session and validates membership for the requested workspace. Clients select a workspace with `X-Workspace-Id`. Omitting the header selects the account’s personal workspace. A separate PostgreSQL role enforces row-level security on application data. Composite foreign keys keep linked records in the same workspace. A sync worker acquires the same workspace context as its owner.

Team members share work records such as tasks, events and CRM relationships. Entries about time, wellbeing, private contact details, pay, notes about people or companies, and personal preferences belong to their author. The team calendar shows each member’s available and planned seconds for a whole day. Personal appointments and their times remain private. A personal plan follows its user across memberships and uses their personal plan limit.

## Consequences

The owner manages source configuration, invitations and backups. Changing the plan preserves stored records. On Free the first two external sources remain eligible for sync and shared edits are limited to the owner. Each account retains its own authentication and sessions.

A personal backup can restore a historical single-owner file. A team export includes shared records and omits private entries, even those written by the owner. Restores keep IDs, discard saved source credentials and run in a transaction. An attempted replacement that would delete private records fails. Changing members, permissions or the tariff is outside the backup format.

-- +goose Up
-- A single existing account owns this installation. Do not assign ambiguous data.
DO $$
BEGIN
    IF (SELECT count(*) FROM users) > 1 THEN
        RAISE EXCEPTION 'Cannot identify the owner of this installation';
    END IF;
END $$;

ALTER TABLE users
    ADD COLUMN email TEXT UNIQUE,
    ADD COLUMN is_legacy BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT users_email_canonical CHECK (email = lower(btrim(email)) AND email <> '');

UPDATE users SET is_legacy = true;
CREATE UNIQUE INDEX users_single_legacy_idx ON users (is_legacy) WHERE is_legacy;
ALTER TABLE users ADD CONSTRAINT users_identity_chk CHECK (is_legacy OR email IS NOT NULL);

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users (id),
    kind TEXT NOT NULL CHECK (kind IN ('personal', 'team')),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    plan TEXT NOT NULL DEFAULT 'free' CHECK (plan IN ('free', 'paid')),
    legacy_paid BOOLEAN NOT NULL DEFAULT false CHECK (NOT legacy_paid OR (kind = 'personal' AND plan = 'paid')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX workspaces_personal_owner_idx ON workspaces (owner_id) WHERE kind = 'personal';

CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id),
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor')),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user_idx ON workspace_members (user_id, workspace_id);
CREATE UNIQUE INDEX workspace_members_owner_idx ON workspace_members (workspace_id) WHERE role = 'owner';

WITH personal AS (
    INSERT INTO workspaces (owner_id, kind, name, plan, legacy_paid)
    SELECT id, 'personal', 'Personal', 'paid', true FROM users
    RETURNING id, owner_id
)
INSERT INTO workspace_members (workspace_id, user_id, role) SELECT id, owner_id, 'owner' FROM personal;

CREATE TABLE account_tokens (
    token_hash TEXT PRIMARY KEY,
    purpose TEXT NOT NULL CHECK (purpose IN ('signup', 'bind', 'reset')),
    email TEXT NOT NULL,
    user_id UUID REFERENCES users (id) ON DELETE CASCADE,
    credential_version INTEGER,
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK ((purpose = 'signup') = (user_id IS NULL)),
    CHECK ((user_id IS NULL) = (credential_version IS NULL))
);
CREATE INDEX account_tokens_expiry_idx ON account_tokens (expires_at);

CREATE TABLE workspace_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (workspace_id, email)
);

CREATE TABLE auth_limits (
    key TEXT PRIMARY KEY,
    attempts INTEGER NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX auth_limits_expiry_idx ON auth_limits (expires_at);
CREATE INDEX sessions_user_idx ON sessions (user_id, expires_at);

-- +goose Down
-- Multi-user data cannot safely be opened with a single-owner server.
DO $$ BEGIN RAISE EXCEPTION 'Restore the pre-upgrade database to downgrade'; END $$;

-- +goose Up
-- Optional preflight uses a private schema. The v48 API still owns the complete public schema.
CREATE FUNCTION pg_temp.wb_v48_ownership(preparing boolean) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    marker constant text := 'Want Brief v48 workspace ownership';
    stage oid := to_regnamespace('wantbrief_v48_ownership');
    creator oid;
    owner_id uuid;
    users_count bigint;
    saved_owner uuid;
    saved_views text[];
    expected_views text[];
    relation record;
    view_name text;
    key_columns text;
    grantee_name text;
BEGIN
    LOCK TABLE public.users IN SHARE MODE;
    SELECT count(*) INTO users_count FROM public.users;
    IF users_count > 1 OR (preparing AND users_count <> 1) THEN
        RAISE EXCEPTION 'Workspace preparation requires exactly one installation owner';
    END IF;
    SELECT id INTO owner_id FROM public.users;
    IF stage IS NULL AND NOT preparing THEN RETURN; END IF;

    IF preparing THEN
        SELECT array_agg(relation_name ORDER BY relation_name) INTO expected_views
        FROM pg_temp.wb_legacy_scope;
    END IF;
    IF stage IS NULL THEN
        CREATE SCHEMA wantbrief_v48_ownership;
        stage := 'wantbrief_v48_ownership'::regnamespace;
        EXECUTE format('COMMENT ON SCHEMA wantbrief_v48_ownership IS %L', marker);
        CREATE TABLE wantbrief_v48_ownership.installation (owner_id uuid NOT NULL, view_names text[] NOT NULL);
        INSERT INTO wantbrief_v48_ownership.installation VALUES (owner_id, expected_views);
        EXECUTE format('COMMENT ON TABLE wantbrief_v48_ownership.installation IS %L', marker);

        FOR relation IN SELECT * FROM pg_temp.wb_legacy_scope LOOP
            SELECT string_agg(format('r.%I', a.attname), ', ' ORDER BY k.position)
                INTO key_columns FROM pg_index i
                CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum,position)
                JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum
                WHERE i.indrelid=format('public.%I',relation.relation_name)::regclass
                  AND i.indisprimary AND k.position<=i.indnkeyatts;
            IF key_columns IS NULL THEN RAISE EXCEPTION 'Missing primary key for %', relation.relation_name; END IF;
            EXECUTE format('CREATE VIEW wantbrief_v48_ownership.%I WITH (security_invoker=true) AS
                SELECT %s, %L::uuid AS workspace_id%s FROM public.%I r OFFSET 0',
                relation.relation_name, key_columns, owner_id,
                CASE WHEN relation.owned THEN format(', %L::uuid AS owner_id',owner_id) ELSE '' END,
                relation.relation_name);
            EXECUTE format('COMMENT ON VIEW wantbrief_v48_ownership.%I IS %L',relation.relation_name,marker);
        END LOOP;

        -- A deployment can grant access through ALTER DEFAULT PRIVILEGES.
        REVOKE ALL ON SCHEMA wantbrief_v48_ownership FROM PUBLIC;
        REVOKE ALL ON ALL TABLES IN SCHEMA wantbrief_v48_ownership FROM PUBLIC;
        FOR grantee_name IN
            SELECT pg_get_userbyid(a.grantee) FROM pg_namespace n, LATERAL aclexplode(n.nspacl) a
                WHERE n.oid=stage AND a.grantee<>n.nspowner AND a.grantee<>0
            UNION
            SELECT pg_get_userbyid(a.grantee) FROM pg_class c, LATERAL aclexplode(c.relacl) a
                WHERE c.relnamespace=stage AND a.grantee<>c.relowner AND a.grantee<>0
        LOOP
            EXECUTE format('REVOKE ALL ON SCHEMA wantbrief_v48_ownership FROM %I',grantee_name);
            EXECUTE format('REVOKE ALL ON ALL TABLES IN SCHEMA wantbrief_v48_ownership FROM %I',grantee_name);
        END LOOP;
        RETURN;
    END IF;

    SELECT nspowner INTO creator FROM pg_namespace WHERE oid=stage;
    IF obj_description(stage,'pg_namespace') IS DISTINCT FROM marker OR NOT pg_has_role(creator,'USAGE')
       OR NOT EXISTS (SELECT FROM pg_class WHERE relnamespace=stage AND relname='installation'
           AND relkind='r' AND relowner=creator AND obj_description(oid,'pg_class')=marker) THEN
        RAISE EXCEPTION 'Reserved workspace preparation schema is already in use';
    END IF;
    SELECT s.owner_id,s.view_names INTO STRICT saved_owner,saved_views FROM wantbrief_v48_ownership.installation s;
    IF owner_id IS DISTINCT FROM saved_owner OR (preparing AND saved_views IS DISTINCT FROM expected_views) THEN
        RAISE EXCEPTION 'Prepared workspace owner or table inventory has changed';
    END IF;
    FOREACH view_name IN ARRAY saved_views LOOP
        IF NOT EXISTS (SELECT FROM pg_class WHERE relnamespace=stage AND relname=view_name
            AND relkind='v' AND relowner=creator AND obj_description(oid,'pg_class')=marker) THEN
            RAISE EXCEPTION 'Workspace preparation view % was replaced', view_name;
        END IF;
        IF NOT preparing THEN
            EXECUTE format('DROP VIEW wantbrief_v48_ownership.%I RESTRICT',view_name);
        END IF;
    END LOOP;
    IF NOT preparing THEN
        DROP TABLE wantbrief_v48_ownership.installation RESTRICT;
        -- Unrelated objects or dependent views abort the whole migration before v49 is recorded.
        DROP SCHEMA wantbrief_v48_ownership RESTRICT;
    END IF;
END $$;
-- end v48 ownership routine
SELECT pg_temp.wb_v48_ownership(false);

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
    INSERT INTO workspaces (id, owner_id, kind, name, plan, legacy_paid)
    SELECT id, id, 'personal', 'Personal', 'paid', true FROM users
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

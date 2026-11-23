-- +goose Up
-- Connections serving application data always assume this non-owner role, including
-- when the installation uses the PostgreSQL container's bootstrap superuser.
CREATE FUNCTION wb_runtime_role() RETURNS name LANGUAGE sql STABLE AS $$
    SELECT ('wb_data_' || oid)::name FROM pg_database WHERE datname=current_database()
$$;
CREATE FUNCTION wb_private_table(tab text) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
    SELECT tab=ANY(ARRAY['notes','time_intervals','stress_logs','person_notes','person_contacts',
       'item_ratings','project_personal_income','person_sites','person_bonds','person_bond_events','person_professions','person_profession_salaries',
       'person_absences','person_addresses','person_birthdays','company_notes','company_titles','company_salaries',
       'company_contracts','earn_days','schedule_settings','schedule_day_overrides','schedule_snapshots'])
$$;
CREATE FUNCTION wb_archiving_team() RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT coalesce(current_setting('wantbrief.backup',true)='shared',false)
$$;
CREATE FUNCTION wb_personal_event(k text, f text, n text, s integer) RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
    SELECT k IN ('timer_log','timer_start','timer_stop') OR f='stress' OR n<>'' OR s IS NOT NULL
$$;
CREATE FUNCTION wb_preserve_private() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME='item_events' THEN
        IF NOT wb_personal_event(OLD.kind, OLD.field, OLD.note, OLD.stress) THEN
            IF TG_OP='DELETE' THEN RETURN OLD; END IF;
            RETURN NEW;
        END IF;
    END IF;
    IF wb_actor() IS NOT NULL THEN
        IF OLD.owner_id<>wb_actor() OR wb_archiving_team() THEN
            RAISE EXCEPTION USING ERRCODE='WB003', MESSAGE='replacement affects private records';
        END IF;
    END IF;
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END $$;

DO $$
DECLARE
    runtime name := wb_runtime_role();
    tab text;
    visible text;
    writable text;
BEGIN
    IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=runtime) THEN
        RAISE EXCEPTION 'Reserved database role % already exists',runtime;
    END IF;
    EXECUTE format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS',runtime);
    EXECUTE format('GRANT %I TO %I',runtime,current_user);
    EXECUTE format('GRANT USAGE ON SCHEMA public TO %I',runtime);
    EXECUTE format('GRANT SELECT ON workspace_members,workspaces,goose_db_version TO %I',runtime);
    FOR tab IN SELECT table_name FROM information_schema.columns
               WHERE table_schema='public' AND column_name='workspace_id'
                 AND table_name NOT IN ('workspace_members','workspace_invitations')
    LOOP
        visible:='workspace_id=wb_workspace() AND EXISTS (SELECT 1 FROM workspace_members m WHERE m.workspace_id=wb_workspace() AND m.user_id=wb_actor())';
        IF wb_private_table(tab) THEN
            visible:=visible || ' AND owner_id=wb_actor() AND NOT wb_archiving_team()';
        ELSIF tab='item_events' THEN
            visible:=visible || ' AND (NOT wb_personal_event(kind,field,note,stress) OR (owner_id=wb_actor() AND NOT wb_archiving_team()))';
        END IF;
        writable:=visible || ' AND EXISTS (SELECT 1 FROM workspaces w WHERE w.id=wb_workspace() AND (w.owner_id=wb_actor() OR w.plan=''paid''))';
        IF tab='sources' THEN
            writable:=writable || ' AND EXISTS (SELECT 1 FROM workspaces w WHERE w.id=wb_workspace() AND w.owner_id=wb_actor())';
        END IF;
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tab);
        EXECUTE format('CREATE POLICY scoped_read ON %I FOR SELECT TO %I USING (%s)',tab,runtime,visible);
        EXECUTE format('CREATE POLICY scoped_insert ON %I FOR INSERT TO %I WITH CHECK (%s)',tab,runtime,writable);
        EXECUTE format('CREATE POLICY scoped_update ON %I FOR UPDATE TO %I USING (%s) WITH CHECK (%s)',tab,runtime,writable,writable);
        EXECUTE format('CREATE POLICY scoped_delete ON %I FOR DELETE TO %I USING (%s)',tab,runtime,writable);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON %I TO %I',tab,runtime);
        IF wb_private_table(tab) OR tab='item_events' THEN
            EXECUTE format('CREATE TRIGGER protect_private BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION wb_preserve_private()',tab);
        END IF;
    END LOOP;
END $$;

CREATE FUNCTION wb_source_quota() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE
    tier text;
BEGIN
    IF NEW.kind='manual' THEN RETURN NEW; END IF;
    SELECT plan INTO STRICT tier FROM workspaces WHERE id=NEW.workspace_id FOR UPDATE;
    IF tier='free' AND (SELECT count(*) FROM sources WHERE workspace_id=NEW.workspace_id AND kind<>'manual' AND id<>NEW.id)>=2 THEN
        RAISE EXCEPTION USING ERRCODE='WB002', MESSAGE='external source limit reached';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER check_source_quota BEFORE INSERT OR UPDATE OF workspace_id,kind ON sources
    FOR EACH ROW EXECUTE FUNCTION wb_source_quota();
CREATE UNIQUE INDEX sources_one_manual_per_workspace ON sources(workspace_id) WHERE kind='manual';

CREATE FUNCTION wb_source_enabled(space uuid, source uuid) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS(SELECT 1 FROM workspaces WHERE id=space AND plan='paid')
       OR source IN (SELECT id FROM sources WHERE workspace_id=space AND kind<>'manual' ORDER BY created_at,id LIMIT 2)
$$;

-- Old record formats combine shared fields with details belonging to the writer.
-- Keep the single-owner representation readable while storing team details by author.
CREATE FUNCTION wb_store_personal_fields() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS(SELECT 1 FROM workspaces WHERE id=NEW.workspace_id AND kind='team') THEN RETURN NEW; END IF;
    IF TG_TABLE_NAME='people' THEN
        IF wb_archiving_team() THEN
            IF NEW.born_on IS NOT NULL THEN RAISE EXCEPTION USING ERRCODE='WB003',MESSAGE='private birthday in shared backup'; END IF;
        ELSIF NEW.born_on IS NOT NULL THEN
            INSERT INTO person_birthdays(workspace_id,owner_id,person_id,born_on) VALUES(NEW.workspace_id,wb_actor(),NEW.id,NEW.born_on)
            ON CONFLICT(workspace_id,owner_id,person_id) DO UPDATE SET born_on=excluded.born_on;
        ELSIF TG_OP='UPDATE' THEN DELETE FROM person_birthdays WHERE person_id=NEW.id AND owner_id=wb_actor();
        END IF;
        NEW.born_on:=NULL;
    ELSIF TG_TABLE_NAME='items' THEN
        IF wb_archiving_team() THEN
            IF NEW.stress IS NOT NULL THEN RAISE EXCEPTION USING ERRCODE='WB003',MESSAGE='private rating in shared backup'; END IF;
        ELSIF NEW.stress IS NOT NULL THEN
            INSERT INTO item_ratings(workspace_id,owner_id,item_id,stress) VALUES(NEW.workspace_id,wb_actor(),NEW.id,NEW.stress)
            ON CONFLICT(workspace_id,owner_id,item_id) DO UPDATE SET stress=excluded.stress;
        ELSIF TG_OP='UPDATE' THEN DELETE FROM item_ratings WHERE item_id=NEW.id AND owner_id=wb_actor();
        END IF;
        NEW.stress:=NULL;
    ELSE
        IF wb_archiving_team() THEN
            IF NEW.monthly_income_usd<>0 OR NEW.monthly_income_rub<>0 OR NEW.monthly_income<>0 THEN
                RAISE EXCEPTION USING ERRCODE='WB003',MESSAGE='private income in shared backup';
            END IF;
        ELSIF NEW.monthly_income_usd<>0 OR NEW.monthly_income_rub<>0 THEN
            INSERT INTO project_personal_income(workspace_id,owner_id,project_id,usd,rub)
            VALUES(NEW.workspace_id,wb_actor(),NEW.id,NEW.monthly_income_usd,NEW.monthly_income_rub)
            ON CONFLICT(workspace_id,owner_id,project_id) DO UPDATE SET usd=excluded.usd,rub=excluded.rub;
        ELSIF TG_OP='UPDATE' THEN DELETE FROM project_personal_income WHERE project_id=NEW.id AND owner_id=wb_actor();
        END IF;
        NEW.monthly_income:=0; NEW.monthly_income_usd:=0; NEW.monthly_income_rub:=0;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER private_birthday BEFORE INSERT OR UPDATE OF born_on ON people FOR EACH ROW EXECUTE FUNCTION wb_store_personal_fields();
CREATE TRIGGER private_rating BEFORE INSERT OR UPDATE OF stress ON items FOR EACH ROW EXECUTE FUNCTION wb_store_personal_fields();
CREATE TRIGGER private_income BEFORE INSERT OR UPDATE OF monthly_income,monthly_income_usd,monthly_income_rub ON projects FOR EACH ROW EXECUTE FUNCTION wb_store_personal_fields();

-- Ordinary writes share the workspace lock; a restore acquires it exclusively.
CREATE FUNCTION wb_guard_write() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock_shared(hashtextextended(wb_workspace()::text,78415));
    RETURN NULL;
END $$;
DO $$
DECLARE tab record;
BEGIN
    FOR tab IN SELECT tablename FROM pg_policies WHERE schemaname='public' AND policyname='scoped_insert' LOOP
        EXECUTE format('CREATE TRIGGER coordinate_restore BEFORE INSERT OR UPDATE OR DELETE ON %I FOR EACH STATEMENT EXECUTE FUNCTION wb_guard_write()',tab.tablename);
    END LOOP;
END $$;

-- +goose Down
DO $$ BEGIN RAISE EXCEPTION 'Restore the pre-upgrade database to downgrade'; END $$;

-- +goose Up
CREATE FUNCTION wb_workspace() RETURNS uuid LANGUAGE sql STABLE AS $$
    SELECT nullif(current_setting('wantbrief.workspace', true), '')::uuid
$$;
CREATE FUNCTION wb_actor() RETURNS uuid LANGUAGE sql STABLE AS $$
    SELECT nullif(current_setting('wantbrief.actor', true), '')::uuid
$$;

-- Only application records from the supported single-owner schema are backfilled.
DO $$
DECLARE
    relations text[] := ARRAY['projects','sources','items','notes','time_intervals','stress_logs','events',
        'project_notes','item_notes','people','person_notes','person_projects','person_events','person_items',
        'person_contacts','person_sites','person_bonds','person_bond_events','person_professions',
        'person_profession_salaries','item_checks','event_overrides','event_notes','schedule_settings',
        'schedule_day_overrides','person_absences','schedule_snapshots','companies','company_projects',
        'company_events','company_people','company_notes','company_titles','company_salaries','company_managers',
        'company_reports','company_contracts','item_events','person_addresses','earn_days'];
    owned text[] := ARRAY['notes','time_intervals','stress_logs','person_notes','person_contacts','person_sites',
        'person_bonds','person_bond_events','person_professions','person_profession_salaries','person_absences',
        'person_addresses','company_notes','company_titles','company_salaries','company_contracts',
        'earn_days','schedule_settings','schedule_day_overrides','schedule_snapshots','item_events'];
    legacy workspaces;
    tab text;
    has_data boolean;
    fk record;
BEGIN
    SELECT * INTO legacy FROM workspaces WHERE legacy_paid;
    FOREACH tab IN ARRAY relations LOOP
        EXECUTE format('ALTER TABLE %I ADD COLUMN workspace_id uuid REFERENCES workspaces(id)',tab);
        IF legacy.id IS NULL THEN
            EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I)',tab) INTO has_data;
            IF has_data THEN RAISE EXCEPTION 'No legacy owner for records in %', tab; END IF;
        ELSE
            EXECUTE format('UPDATE %I SET workspace_id=$1',tab) USING legacy.id;
        END IF;
        EXECUTE format('ALTER TABLE %I ALTER workspace_id SET NOT NULL, ALTER workspace_id SET DEFAULT wb_workspace()',tab);
        IF tab=ANY(owned) THEN
            EXECUTE format('ALTER TABLE %I ADD COLUMN owner_id uuid REFERENCES users(id)',tab);
            EXECUTE format('UPDATE %I SET owner_id=$1',tab) USING legacy.owner_id;
            EXECUTE format('ALTER TABLE %I ALTER owner_id SET NOT NULL, ALTER owner_id SET DEFAULT wb_actor()',tab);
        END IF;
        IF EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=tab::regclass AND attname='id' AND atttypid='uuid'::regtype AND NOT attisdropped) THEN
            EXECUTE format('ALTER TABLE %I ADD UNIQUE (workspace_id,id)',tab);
        ELSE
            EXECUTE format('CREATE INDEX ON %I(workspace_id)',tab);
        END IF;
    END LOOP;

    -- Compound keys also check writes which only receive a parent ID in the HTTP path.
    FOR fk IN SELECT c.conrelid::regclass AS child, c.confrelid::regclass AS parent,
                 c.conname, a.attname AS col, c.confdeltype
              FROM pg_constraint c JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=c.conkey[1]
              WHERE c.contype='f' AND c.conrelid::regclass::text=ANY(relations)
                AND c.confrelid::regclass::text=ANY(relations) AND cardinality(c.conkey)=1
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I, ADD FOREIGN KEY (workspace_id,%I) REFERENCES %s(workspace_id,id) ON DELETE %s',
            fk.child,fk.conname,fk.col,fk.parent,
            CASE fk.confdeltype WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN format('SET NULL (%I)',fk.col) ELSE 'NO ACTION' END);
    END LOOP;
END $$;

ALTER TABLE schedule_settings DROP CONSTRAINT schedule_settings_pkey, ADD PRIMARY KEY(workspace_id,owner_id,id);
ALTER TABLE schedule_day_overrides DROP CONSTRAINT schedule_day_overrides_pkey, ADD PRIMARY KEY(workspace_id,owner_id,day);
ALTER TABLE schedule_snapshots DROP CONSTRAINT schedule_snapshots_pkey, ADD PRIMARY KEY(workspace_id,owner_id,kind);
ALTER TABLE earn_days DROP CONSTRAINT earn_days_pkey, ADD PRIMARY KEY(workspace_id,owner_id,day);

DROP INDEX company_salaries_open_idx;
CREATE UNIQUE INDEX company_salaries_open_idx ON company_salaries(owner_id,company_id,currency) WHERE ended_on IS NULL;
DROP INDEX company_titles_open_idx;
CREATE UNIQUE INDEX company_titles_open_idx ON company_titles(owner_id,company_id) WHERE ended_on IS NULL;
DROP INDEX company_contracts_me_open_idx;
CREATE UNIQUE INDEX company_contracts_me_open_idx ON company_contracts(owner_id,company_id) WHERE person_id IS NULL AND ended_on IS NULL;
DROP INDEX company_contracts_person_open_idx;
CREATE UNIQUE INDEX company_contracts_person_open_idx ON company_contracts(owner_id,company_id,person_id) WHERE person_id IS NOT NULL AND ended_on IS NULL;
DROP INDEX person_bonds_open_pair_idx;
CREATE UNIQUE INDEX person_bonds_open_pair_idx ON person_bonds(owner_id,COALESCE(person_a_id,'00000000-0000-0000-0000-000000000000'::uuid),person_b_id) WHERE ended_on IS NULL;
ALTER TABLE person_professions ADD UNIQUE(workspace_id,owner_id,id);
ALTER TABLE person_profession_salaries ADD FOREIGN KEY(workspace_id,owner_id,profession_id) REFERENCES person_professions(workspace_id,owner_id,id) ON DELETE CASCADE;
ALTER TABLE person_bonds ADD UNIQUE(workspace_id,owner_id,id);
ALTER TABLE person_bond_events ADD FOREIGN KEY(workspace_id,owner_id,bond_id) REFERENCES person_bonds(workspace_id,owner_id,id) ON DELETE CASCADE;

CREATE TABLE person_birthdays (
    workspace_id uuid NOT NULL DEFAULT wb_workspace(),
    owner_id uuid NOT NULL DEFAULT wb_actor() REFERENCES users(id),
    person_id uuid NOT NULL, born_on date NOT NULL,
    PRIMARY KEY (workspace_id, owner_id, person_id),
    FOREIGN KEY (workspace_id,person_id) REFERENCES people(workspace_id,id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE item_ratings (
    workspace_id uuid NOT NULL DEFAULT wb_workspace(), owner_id uuid NOT NULL DEFAULT wb_actor() REFERENCES users(id),
    item_id uuid NOT NULL, stress smallint NOT NULL CHECK(stress BETWEEN 1 AND 5),
    PRIMARY KEY(workspace_id,owner_id,item_id),
    FOREIGN KEY(workspace_id,item_id) REFERENCES items(workspace_id,id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE project_personal_income (
    workspace_id uuid NOT NULL DEFAULT wb_workspace(), owner_id uuid NOT NULL DEFAULT wb_actor() REFERENCES users(id),
    project_id uuid NOT NULL, usd numeric(12,2) NOT NULL CHECK(usd>=0), rub numeric(12,2) NOT NULL CHECK(rub>=0),
    PRIMARY KEY(workspace_id,owner_id,project_id),
    FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);

ALTER TABLE sources DROP CONSTRAINT sources_kind_check,
    ADD CHECK (kind IN ('jira','todoist','manual','gitlab','telegram','slack','whoop'));

-- Older single-owner starts supplied these defaults when an import contained none.
INSERT INTO sources(workspace_id,kind,name)
    SELECT w.id,'manual','Manual' FROM workspaces w WHERE w.legacy_paid
    AND NOT EXISTS(SELECT FROM sources s WHERE s.workspace_id=w.id AND s.kind='manual');
INSERT INTO projects(workspace_id,name,color,target_hours_day,target_hours_week)
    SELECT w.id,v.name,v.color,v.hours/7.0,v.hours FROM workspaces w
    CROSS JOIN (VALUES ('Work','#4C4CFF',30),('Life','#7C8CFF',10)) v(name,color,hours)
    WHERE w.legacy_paid AND NOT EXISTS(SELECT FROM projects p WHERE p.workspace_id=w.id);

-- +goose Down
DO $$ BEGIN RAISE EXCEPTION 'Restore the pre-upgrade database to downgrade'; END $$;

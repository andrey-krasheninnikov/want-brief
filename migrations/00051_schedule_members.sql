-- +goose Up
ALTER TABLE items ADD COLUMN assignee_user_id UUID,
    ADD CONSTRAINT items_workspace_assignee_fk FOREIGN KEY (workspace_id, assignee_user_id)
        REFERENCES workspace_members (workspace_id, user_id) ON DELETE SET NULL (assignee_user_id);
CREATE INDEX items_assignee_idx ON items (workspace_id, assignee_user_id)
    WHERE assignee_user_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE event_participants (
    workspace_id UUID NOT NULL DEFAULT wb_workspace(),
    event_id UUID NOT NULL,
    user_id UUID NOT NULL,
    PRIMARY KEY (workspace_id, event_id, user_id),
    FOREIGN KEY (workspace_id, event_id) REFERENCES events (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);

CREATE TABLE company_employment_dates (
    workspace_id uuid NOT NULL DEFAULT wb_workspace(), owner_id uuid NOT NULL DEFAULT wb_actor() REFERENCES users(id),
    company_id uuid NOT NULL, started_on date, ended_on date,
    CHECK(ended_on IS NULL OR (started_on IS NOT NULL AND ended_on>=started_on)),
    PRIMARY KEY(workspace_id,owner_id,company_id),
    FOREIGN KEY(workspace_id,company_id) REFERENCES companies(workspace_id,id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);
DROP INDEX company_managers_open_idx;
CREATE UNIQUE INDEX company_managers_open_idx ON company_managers(owner_id,company_id) WHERE ended_on IS NULL;
DROP INDEX company_reports_open_idx;
CREATE UNIQUE INDEX company_reports_open_idx ON company_reports(owner_id,company_id,person_id) WHERE ended_on IS NULL;

-- +goose Down
DO $$ BEGIN RAISE EXCEPTION 'Restore the pre-upgrade database to downgrade'; END $$;

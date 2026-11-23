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

-- +goose Down
DROP TABLE event_participants;
ALTER TABLE items DROP COLUMN assignee_user_id;

-- +goose Up
CREATE TABLE chairman_permissions (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    granted_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO chairman_permissions(user_id)
SELECT DISTINCT user_id FROM memberships WHERE role='CHAIRMAN' AND status='ACTIVE';

-- +goose Down
DROP TABLE chairman_permissions;

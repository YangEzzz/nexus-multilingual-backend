CREATE TABLE IF NOT EXISTS project_history_logs (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    action VARCHAR(500) NOT NULL,
    item_count INTEGER NOT NULL DEFAULT 1,
    term_keys JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_project_history_logs_project
        FOREIGN KEY (project_id) REFERENCES i18n_projects (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_history_logs_project_id
    ON project_history_logs (project_id);

CREATE INDEX IF NOT EXISTS idx_project_history_logs_user_id
    ON project_history_logs (user_id);

CREATE INDEX IF NOT EXISTS idx_project_history_logs_created_at
    ON project_history_logs (created_at DESC);

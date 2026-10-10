CREATE TABLE IF NOT EXISTS locale_releases (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES i18n_projects(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    data JSONB NOT NULL,
    published_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT idx_locale_release_version UNIQUE (project_id, version)
);

CREATE TABLE IF NOT EXISTS locale_release_files (
    id BIGSERIAL PRIMARY KEY,
    release_id BIGINT NOT NULL REFERENCES locale_releases(id) ON DELETE CASCADE,
    language_code VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    CONSTRAINT idx_locale_release_file UNIQUE (release_id, language_code)
);

CREATE INDEX IF NOT EXISTS idx_locale_release_files_language_code
    ON locale_release_files(language_code);

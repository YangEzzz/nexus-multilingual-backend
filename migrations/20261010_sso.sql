CREATE TABLE IF NOT EXISTS sso_login_transactions (
 id varchar(64) PRIMARY KEY, browser_hash varchar(64) NOT NULL,
 verifier varchar(128) NOT NULL, return_path text NOT NULL, mode varchar(16) NOT NULL,
 status varchar(16) NOT NULL, message text,
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sso_login_transactions_expiry ON sso_login_transactions(expires_at);
CREATE TABLE IF NOT EXISTS auth_sessions (
 id varchar(64) PRIMARY KEY, user_id bigint NOT NULL REFERENCES users(id), external_id varchar(100) NOT NULL,
 verified_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions(expires_at);

CREATE TABLE auth_sessions (
    code TEXT PRIMARY KEY,
    verification TEXT NOT NULL,
    token TEXT,
    user_id TEXT,
    email TEXT,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_auth_sessions_created_at ON auth_sessions (created_at);

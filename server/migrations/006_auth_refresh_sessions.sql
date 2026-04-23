ALTER TABLE auth_sessions
    ADD COLUMN code_challenge TEXT,
    ADD COLUMN challenge_method TEXT,
    ADD COLUMN approved_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN approved_at TIMESTAMPTZ;

CREATE INDEX idx_auth_sessions_approved_at ON auth_sessions (approved_at);

CREATE TABLE refresh_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id UUID NOT NULL,
    secret_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    rotated_from UUID REFERENCES refresh_sessions(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT
);

CREATE INDEX idx_refresh_sessions_user_id ON refresh_sessions (user_id);
CREATE INDEX idx_refresh_sessions_family_id ON refresh_sessions (family_id);
CREATE INDEX idx_refresh_sessions_expires_at ON refresh_sessions (expires_at);

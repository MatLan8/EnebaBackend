CREATE TABLE refresh_tokens (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash             TEXT        NOT NULL UNIQUE, -- SHA-256 hex of the raw token
    expires_at             TIMESTAMPTZ NOT NULL,
    revoked_at             TIMESTAMPTZ,                 -- NULL = still active
    replaced_by_token_hash TEXT,                        -- set when rotated
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Used for "revoke all tokens of this user" on reuse detection.
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE TABLE IF NOT EXISTS verification_token_ (
    hash_ BYTEA PRIMARY KEY,
    expiry_ TIMESTAMPTZ NOT NULL,
    scope_ TEXT NOT NULL,
    email_ CITEXT NOT NULL,
    user_id_ uuid REFERENCES user_(id_) ON DELETE CASCADE
);

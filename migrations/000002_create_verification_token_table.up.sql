CREATE TABLE IF NOT EXISTS verification_token_ (
    hash_ BYTEA PRIMARY KEY,
    email_ CITEXT NOT NULL,
    expiry_ TIMESTAMPTZ NOT NULL
);
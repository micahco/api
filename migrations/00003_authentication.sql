-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS authentication_token_ (
    hash_ BYTEA PRIMARY KEY,
    expiry_ TIMESTAMPTZ NOT NULL,
    user_id_ uuid NOT NULL REFERENCES user_ ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS authentication_token_;
-- +goose StatementEnd

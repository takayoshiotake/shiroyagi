CREATE TABLE mail_accounts (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    mail_address TEXT NOT NULL,
    mail_username TEXT NOT NULL,
    encrypted_password_blob BYTEA NOT NULL,
    encrypted_password_version SMALLINT NOT NULL,
    wrapped_dek BYTEA NOT NULL,
    kek_version SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_mail_accounts_user_id ON mail_accounts(user_id);

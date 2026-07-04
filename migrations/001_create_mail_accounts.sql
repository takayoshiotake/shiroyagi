CREATE TABLE mail_accounts (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    email_address TEXT NOT NULL,
    imap_host TEXT NOT NULL,
    imap_port INTEGER NOT NULL,
    imap_security TEXT NOT NULL,
    imap_username TEXT NOT NULL,
    encrypted_imap_password BYTEA NOT NULL,
    wrapped_dek BYTEA NOT NULL,
    kek_version SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_mail_accounts_user_id ON mail_accounts(user_id);

CREATE TABLE mail_accounts (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    email_address TEXT NOT NULL,
    wrapped_dek BYTEA NOT NULL,
    kek_version SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT uk_mail_accounts_user_id_email_address UNIQUE (user_id, email_address)
);

CREATE TABLE imap_accounts (
    mail_account_id UUID PRIMARY KEY REFERENCES mail_accounts(id) ON DELETE CASCADE,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    security TEXT NOT NULL,
    username TEXT NOT NULL,
    encrypted_password BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE smtp_accounts (
    mail_account_id UUID PRIMARY KEY REFERENCES mail_accounts(id) ON DELETE CASCADE,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    security TEXT NOT NULL,
    username TEXT NOT NULL,
    encrypted_password BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_mail_accounts_user_id ON mail_accounts(user_id);

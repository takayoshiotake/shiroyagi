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

INSERT INTO imap_accounts (
    mail_account_id,
    host,
    port,
    security,
    username,
    encrypted_password,
    created_at,
    updated_at
)
SELECT
    id,
    imap_host,
    imap_port,
    imap_security,
    imap_username,
    encrypted_imap_password,
    created_at,
    updated_at
FROM mail_accounts;

INSERT INTO smtp_accounts (
    mail_account_id,
    host,
    port,
    security,
    username,
    encrypted_password,
    created_at,
    updated_at
)
SELECT
    id,
    smtp_host,
    smtp_port,
    smtp_security,
    smtp_username,
    encrypted_smtp_password,
    created_at,
    updated_at
FROM mail_accounts
WHERE smtp_host <> ''
  AND length(encrypted_smtp_password) > 0;

ALTER TABLE mail_accounts
    DROP COLUMN imap_host,
    DROP COLUMN imap_port,
    DROP COLUMN imap_security,
    DROP COLUMN imap_username,
    DROP COLUMN encrypted_imap_password,
    DROP COLUMN smtp_host,
    DROP COLUMN smtp_port,
    DROP COLUMN smtp_security,
    DROP COLUMN smtp_username,
    DROP COLUMN encrypted_smtp_password;

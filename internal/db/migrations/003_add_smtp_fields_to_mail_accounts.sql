ALTER TABLE mail_accounts
    ADD COLUMN smtp_host TEXT NOT NULL DEFAULT '',
    ADD COLUMN smtp_port INTEGER NOT NULL DEFAULT 587,
    ADD COLUMN smtp_security TEXT NOT NULL DEFAULT 'starttls',
    ADD COLUMN smtp_username TEXT NOT NULL DEFAULT '',
    ADD COLUMN encrypted_smtp_password BYTEA NOT NULL DEFAULT '\x'::bytea;

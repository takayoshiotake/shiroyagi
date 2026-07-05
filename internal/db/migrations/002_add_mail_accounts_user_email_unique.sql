ALTER TABLE mail_accounts
    ADD CONSTRAINT uk_mail_accounts_user_id_email_address UNIQUE (user_id, email_address);

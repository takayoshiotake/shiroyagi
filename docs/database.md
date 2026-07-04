# Database

Shiroyagi separates application users from mail accounts.

- `users`: Shiroyagi application users authenticated by Keycloak/OIDC
- `user_preferences`: user-wide UI and behavior preferences
- `mail_accounts`: IMAP account connection credentials
- `mail_account_settings`: per-mail-account display and sending settings
- `schema_migrations`: applied database migration versions

## schema_migrations

`schema_migrations` records migration files that have already been applied.

Migration SQL files are embedded in the Shiroyagi binary from
`internal/db/migrations`. On startup, Shiroyagi creates this table if needed,
sorts the embedded `*.sql` files by name, applies files that are not recorded
yet, and then inserts each applied filename as the migration version.

```text
version
applied_at
```

### Columns

- `version`: migration filename, such as `001_create_mail_accounts.sql`
- `applied_at`: time the migration was recorded

## users

`users` represents Shiroyagi users, not mail accounts.

Shiroyagi trusts a single configured OIDC issuer, typically Keycloak. External identity providers such as Google or Microsoft Entra ID should be integrated into Keycloak, so the application only stores the Keycloak subject.

```text
id
subject
role
created_at
updated_at
```

### Columns

- `id`: Shiroyagi user ID
- `subject`: OIDC `sub` claim from Keycloak
- `role`: application role such as `user` or `admin`

## user_preferences

`user_preferences` stores settings that apply to the whole Shiroyagi user.

```text
user_id
theme
language
timezone
created_at
updated_at
```

### Columns

- `user_id`: references `users.id`
- `theme`: UI theme preference
- `language`: UI language
- `timezone`: display timezone

## mail_accounts

`mail_accounts` stores connection information for IMAP accounts.

One Shiroyagi user can have multiple mail accounts.

```text
id
user_id
email_address
imap_host
imap_port
imap_security
imap_username
encrypted_imap_password
wrapped_dek
kek_version
created_at
updated_at
```

### Columns

- `id`: mail account ID
- `user_id`: references `users.id`
- `email_address`: email address for this account
- `imap_host`: IMAP server hostname
- `imap_port`: IMAP server port
- `imap_security`: IMAP transport security mode, such as `tls`, `starttls`, or `none`
- `imap_username`: IMAP login username
- `encrypted_imap_password`: encrypted IMAP password
- `wrapped_dek`: DEK encrypted with the current KEK
- `kek_version`: KEK version used to wrap the DEK

### Encryption

IMAP passwords are encrypted with envelope encryption.

- Data encryption: AES-256-GCM
- DEK wrapping: AES-256-GCM
- KEK is loaded from `/run/secrets/mail_account_kek`
- KEK is never stored in PostgreSQL

`encrypted_password_version` is intentionally not stored. AES-256-GCM is the only supported encryption format for now. If a future format change becomes necessary, the encrypted blob can be versioned or migrated then.

## mail_account_settings

`mail_account_settings` stores settings that belong to a specific mail account.

```text
mail_account_id
display_name
signature
reply_to
created_at
updated_at
```

### Columns

- `mail_account_id`: references `mail_accounts.id`
- `display_name`: display name used when sending mail
- `signature`: default signature for this mail account
- `reply_to`: optional Reply-To address

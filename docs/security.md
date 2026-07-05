# Security

## OIDC

Validate:

- iss
- aud
- exp
- nbf
- signature via JWKS
- allowed alg

Do not trust arbitrary issuers.

## Mail Passwords

Use envelope encryption.

```text
IMAP password -> DEK -> AES-256-GCM -> imap_accounts.encrypted_password
SMTP password -> DEK -> AES-256-GCM -> smtp_accounts.encrypted_password
DEK           -> KEK -> AES-256-GCM -> wrapped_dek
```

KEK is stored outside PostgreSQL.

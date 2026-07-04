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
IMAP password -> DEK -> AES-256-GCM -> encrypted_imap_password
other secrets -> DEK -> AES-256-GCM -> encrypted_* fields
DEK           -> KEK -> AES-256-GCM -> wrapped_dek
```

KEK is stored outside PostgreSQL.

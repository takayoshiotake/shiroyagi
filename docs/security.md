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
mail_password -> DEK -> AES-256-GCM -> encrypted_password_blob
DEK -> KEK -> wrapped_dek
```

KEK is stored outside PostgreSQL.

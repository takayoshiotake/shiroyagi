# Development

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

Required secrets:

```text
secrets/dev/postgres_password
secrets/dev/mail_account_kek
secrets/dev/oidc_client_secret
```

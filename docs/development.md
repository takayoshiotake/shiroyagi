# Development

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

Required secrets:

```text
secrets/dev/postgres_password
secrets/dev/master_key
secrets/dev/oidc_client_secret
secrets/dev/keycloak_admin_password
```

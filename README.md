# Webmail Project

Self-hosted webmail for on-premise mail servers.

## Start development

Create dev secret files first:

```bash
mkdir -p secrets/dev
printf 'shiroyagi' > secrets/dev/postgres_password
printf 'dev-master-key-32bytes-change-me!!' > secrets/dev/master_key
printf 'dev-oidc-client-secret' > secrets/dev/oidc_client_secret
printf 'admin' > secrets/dev/keycloak_admin_password
```

Then run:

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

URLs:

- Web: http://localhost:8080
- Keycloak: http://localhost:8081
- Mailpit: http://localhost:8025

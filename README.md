# Webmail Project

Self-hosted webmail for on-premise mail servers.

## Start development

Create dev secret files first:

```bash
mkdir -p secrets/dev
printf 'shiroyagi' > secrets/dev/postgres_password
openssl rand 32 > secrets/dev/master_key
printf 'dev-oidc-client-secret' > secrets/dev/oidc_client_secret
```

Then run:

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

Keycloak imports the development realm automatically on startup. The `dev`
realm, `shiroyagi` OIDC client, and `dev` user are created from
`keycloak/realms/dev-realm.json`.
The development app login is `dev` / `dev`. The Keycloak admin console login
is `admin` / `admin`.

## Development layout

```mermaid
flowchart LR
  browser[Browser] --> web[Webmail :8080]
  web --> postgres[(PostgreSQL)]
  web --> dovecot[Dovecot IMAP]
  web --> mailpit[Mailpit SMTP]
  web --> keycloak[Keycloak OIDC :8081]
  browser --> keycloak
  browser --> mailpitUI[Mailpit UI :8025]
```

URLs:

- Web: http://localhost:8080
- Keycloak: http://localhost:8081
- Mailpit: http://localhost:8025

Manual OIDC login check:

```text
http://localhost:8081/realms/dev/protocol/openid-connect/auth?client_id=shiroyagi&redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fauth%2Fcallback&response_type=code&scope=openid
```

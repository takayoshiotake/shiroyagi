# Shiroyagi

Self-hosted webmail for on-premise mail servers.

## Start development

Create dev secret files first:

```bash
mkdir -p secrets/dev
printf 'shiroyagi' > secrets/dev/postgres_password
openssl rand 32 > secrets/dev/mail_account_kek
printf 'dev-oidc-client-secret' > secrets/dev/oidc_client_secret
```

Then run:

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

To run only the Go app locally against the development services:

```bash
OIDC_ISSUER=http://localhost:8081/realms/dev \
OIDC_BROWSER_ISSUER=http://localhost:8081/realms/dev \
OIDC_CLIENT_ID=shiroyagi \
OIDC_CLIENT_SECRET_FILE=secrets/dev/oidc_client_secret \
OIDC_REDIRECT_URI=http://localhost:8080/auth/callback \
DATABASE_HOST=localhost \
DATABASE_PORT=5432 \
DATABASE_NAME=shiroyagi \
DATABASE_USER=shiroyagi \
DATABASE_PASSWORD_FILE=secrets/dev/postgres_password \
MAIL_ACCOUNT_KEK_FILE=secrets/dev/mail_account_kek \
MAIL_ACCOUNT_KEK_VERSION=1 \
go run ./cmd/shiroyagi
```

`MAIL_ACCOUNT_KEK_VERSION` selects the KEK version used for newly saved mail
account secrets. Development uses `1`.

Keycloak imports the development realm automatically on startup. The `dev`
realm, `shiroyagi` OIDC client, and `dev` user are created from
`dev/keycloak/realm.json`.
The development app login is `dev` / `dev`. The Keycloak admin console login
is `admin` / `admin`.

## Development layout

```mermaid
flowchart LR
  browser[Browser] --> web[Shiroyagi :8080]
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

Login check:

```text
http://localhost:8080/signin
```

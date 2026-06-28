# AGENTS.md

## Project

Self-hosted webmail application for on-premise mail servers.

## Tech Stack

- Go backend
- Go html/template
- HTMX
- SSE later
- PostgreSQL
- OIDC / Keycloak
- Dovecot for development IMAP
- Mailpit for development SMTP

## Development

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

or:

```bash
docker compose -f compose.yaml -f compose.dev.yaml up
```

## Security

- Never store plaintext mail passwords.
- Use envelope encryption.
- KEK is loaded from `/run/secrets/master_key`.
- Do not log secrets or full JWTs.
- Reject local auth in production.

# Architecture

## Development

```text
Browser
  ├── http://localhost:8080 -> Web
  └── http://keycloak.localhost:8081 -> Keycloak

Web
  ├── PostgreSQL
  └── Stalwart (IMAP and SMTP, local delivery for example.test)
```

## Production

```text
Internet
  -> Caddy
  -> Web

Web
  -> External OIDC Provider
  -> External on-premise mail server
  -> PostgreSQL
```

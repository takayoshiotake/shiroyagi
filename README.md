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

Windows users can run these commands from WSL.

### Run the complete development environment

Run all development services, including the Go application, in containers:

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

### Run the Go application locally

Start the complete development environment in the background:

```bash
podman compose -f compose.yaml -f compose.dev.yaml up -d
```

Stop the containerized Go application:

```bash
podman compose -f compose.yaml -f compose.dev.yaml stop web
```

Then run the Go application on the host.

**macOS and Linux**

```bash
./scripts/run-dev.sh
```

**Windows PowerShell**

```powershell
.\scripts\run-dev.ps1
```

> [!NOTE]
> `MAIL_ACCOUNT_KEK_VERSION` selects the KEK version used to encrypt newly saved mail account secrets. Development uses version `1`.
>
> Keycloak automatically imports the development realm on startup. The `dev` realm, `shiroyagi` OIDC client, and `dev` user are defined in `dev/keycloak/realm.json`.
>
> The development app login is `dev` / `dev`. The Keycloak admin console login is `admin` / `admin`.

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

IMAP check:

1. Sign in and open `Mail accounts`.
2. Create a mail account.
3. Open `IMAP` for that account and save the IMAP connection settings.
   Use `dev@example.test` as the email address and IMAP username. Use `dev` as
   the IMAP password.
   When the Go app is run locally against the development services, use
   `localhost`, port `2143`, and protocol `IMAP` for the development Dovecot
   service. When the app runs in compose, use `dovecot`, port `31143`, and
   protocol `IMAP`.
4. Open `Inbox` from the mail account list.

The first IMAP link opens `INBOX` and fetches the latest 100 messages.
Opening a message fetches and displays its inline text body.

SMTP check:

1. Sign in and open `Mail accounts`.
2. Create or reuse a mail account.
3. Open `SMTP` for that account and save the SMTP connection settings.
   Use protocol `Plain`, SMTP username `dev@example.test`, and SMTP password
   `dev`.
   When the Go app is run locally against the development services, use
   `localhost` and port `1025`. When the app runs in compose, use `mailpit`
   and port `1025`.
4. Open `Send test` from the mail account list.
5. Enter a recipient, subject, and body, then send the message.
6. Open Mailpit at http://localhost:8025 and confirm the message.

## Build

```bash
make build
```

This creates the `shiroyagi` binary and writes Go module, dependency, build setting, and VCS metadata to `shiroyagi-build-info.txt`.

The binary version defaults to `git describe --tags --always --dirty`.

```bash
./shiroyagi --version
```

## Release build

To create a release archive for the current operating system and architecture:

```bash
make release
```

To build an archive for a specific target platform, set `GOOS` and `GOARCH`:

```bash
make release GOOS=linux GOARCH=amd64
```

Generated archives and their SHA-256 checksums are written to `dist/`:

```text
dist/
├── shiroyagi_v0.1.0_linux_amd64.tar.gz
└── checksums.txt
```

Release binaries are built with `CGO_ENABLED=0` to avoid runtime dependencies on the target system’s C library, such as `glibc`.

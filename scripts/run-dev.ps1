$ErrorActionPreference = "Stop"

$env:OIDC_ISSUER = "http://keycloak.localhost:8081/realms/dev"
$env:OIDC_CLIENT_ID = "shiroyagi"
$env:OIDC_CLIENT_SECRET_FILE = "secrets/dev/oidc_client_secret"
$env:OIDC_REDIRECT_URI = "http://localhost:8080/auth/callback"

$env:DATABASE_HOST = "localhost"
$env:DATABASE_PORT = "5432"
$env:DATABASE_NAME = "shiroyagi"
$env:DATABASE_USER = "shiroyagi"
$env:DATABASE_PASSWORD_FILE = "secrets/dev/postgres_password"

$env:MAIL_ACCOUNT_KEK_FILE = "secrets/dev/mail_account_kek"
$env:MAIL_ACCOUNT_KEK_VERSION = "1"

go run ./cmd/shiroyagi @args

if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

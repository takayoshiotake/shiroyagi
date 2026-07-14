#!/usr/bin/env bash
set -euo pipefail

export OIDC_ISSUER="http://localhost:8081/realms/dev"
export OIDC_BROWSER_ISSUER="http://localhost:8081/realms/dev"
export OIDC_CLIENT_ID="shiroyagi"
export OIDC_CLIENT_SECRET_FILE="secrets/dev/oidc_client_secret"
export OIDC_REDIRECT_URI="http://localhost:8080/auth/callback"

export DATABASE_HOST="localhost"
export DATABASE_PORT="5432"
export DATABASE_NAME="shiroyagi"
export DATABASE_USER="shiroyagi"
export DATABASE_PASSWORD_FILE="secrets/dev/postgres_password"

export MAIL_ACCOUNT_KEK_FILE="secrets/dev/mail_account_kek"
export MAIL_ACCOUNT_KEK_VERSION="1"

go run ./cmd/shiroyagi "$@"

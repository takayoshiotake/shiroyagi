package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesConfiguredMailAccountKEKVersion(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MAIL_ACCOUNT_KEK_VERSION", "7")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MailCrypto.KEKVersion != 7 {
		t.Fatalf("MailCrypto.KEKVersion = %d, want 7", cfg.MailCrypto.KEKVersion)
	}
}

func TestLoadDefaultsMailAccountKEKVersion(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MailCrypto.KEKVersion != 1 {
		t.Fatalf("MailCrypto.KEKVersion = %d, want 1", cfg.MailCrypto.KEKVersion)
	}
}

func TestLoadRejectsInvalidMailAccountKEKVersion(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MAIL_ACCOUNT_KEK_VERSION", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "MAIL_ACCOUNT_KEK_VERSION") {
		t.Fatalf("Load() error = %q, want MAIL_ACCOUNT_KEK_VERSION", err)
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()

	t.Setenv("OIDC_ISSUER", "http://issuer.example.test")
	t.Setenv("OIDC_CLIENT_ID", "shiroyagi")
	t.Setenv("OIDC_REDIRECT_URI", "http://localhost:8080/auth/callback")
	t.Setenv("DATABASE_HOST", "localhost")
	t.Setenv("DATABASE_PORT", "5432")
	t.Setenv("DATABASE_NAME", "shiroyagi")
	t.Setenv("DATABASE_USER", "shiroyagi")
	t.Setenv("MAIL_ACCOUNT_KEK_FILE", filepath.Join(t.TempDir(), "mail_account_kek"))

	oidcSecret := writeSecret(t, "oidc_client_secret", "secret")
	databasePassword := writeSecret(t, "postgres_password", "password")
	t.Setenv("OIDC_CLIENT_SECRET_FILE", oidcSecret)
	t.Setenv("DATABASE_PASSWORD_FILE", databasePassword)
}

func writeSecret(t *testing.T, name, value string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	return path
}

package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Issuer        string
	BrowserIssuer string
	ClientID      string
	ClientSecret  string
	RedirectURI   string
	Database      DatabaseConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func Load() (Config, error) {
	cfg := Config{
		Issuer:        os.Getenv("OIDC_ISSUER"),
		BrowserIssuer: os.Getenv("OIDC_BROWSER_ISSUER"),
		ClientID:      os.Getenv("OIDC_CLIENT_ID"),
		RedirectURI:   os.Getenv("OIDC_REDIRECT_URI"),
		Database: DatabaseConfig{
			Host: os.Getenv("DATABASE_HOST"),
			Port: os.Getenv("DATABASE_PORT"),
			Name: os.Getenv("DATABASE_NAME"),
			User: os.Getenv("DATABASE_USER"),
		},
	}
	if cfg.BrowserIssuer == "" {
		cfg.BrowserIssuer = cfg.Issuer
	}

	var missing []string
	if cfg.Issuer == "" {
		missing = append(missing, "OIDC_ISSUER")
	}
	if cfg.ClientID == "" {
		missing = append(missing, "OIDC_CLIENT_ID")
	}
	if cfg.RedirectURI == "" {
		missing = append(missing, "OIDC_REDIRECT_URI")
	}
	if cfg.Database.Host == "" {
		missing = append(missing, "DATABASE_HOST")
	}
	if cfg.Database.Port == "" {
		missing = append(missing, "DATABASE_PORT")
	}
	if cfg.Database.Name == "" {
		missing = append(missing, "DATABASE_NAME")
	}
	if cfg.Database.User == "" {
		missing = append(missing, "DATABASE_USER")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	clientSecret, err := readSecretFile("OIDC_CLIENT_SECRET_FILE")
	if err != nil {
		return Config{}, err
	}
	cfg.ClientSecret = clientSecret

	databasePassword, err := readSecretFile("DATABASE_PASSWORD_FILE")
	if err != nil {
		return Config{}, err
	}
	cfg.Database.Password = databasePassword

	return cfg, nil
}

func readSecretFile(envName string) (string, error) {
	path := os.Getenv(envName)
	if path == "" {
		return "", fmt.Errorf("missing required environment variable: %s", envName)
	}

	value, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", envName, err)
	}
	secret := strings.TrimSpace(string(value))
	if secret == "" {
		return "", fmt.Errorf("secret file configured by %s is empty", envName)
	}
	return secret, nil
}

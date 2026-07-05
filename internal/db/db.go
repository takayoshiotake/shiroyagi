package db

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"

	"github.com/takayoshiotake/shiroyagi/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   cfg.Name,
	}
	query := dsn.Query()
	query.Set("sslmode", "disable")
	dsn.RawQuery = query.Encode()

	database, err := sql.Open("pgx", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return database, nil
}

package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func Wait(ctx context.Context, database *sql.DB, timeout time.Duration) error {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		if err := database.PingContext(deadline); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-deadline.Done():
			return fmt.Errorf("database did not become ready: %w", lastErr)
		case <-ticker.C:
		}
	}
}

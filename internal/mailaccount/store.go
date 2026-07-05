package mailaccount

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrDuplicateAccount = errors.New("duplicate mail account")

type Store struct {
	db *sql.DB
}

type Account struct {
	ID                    string
	UserID                string
	EmailAddress          string
	IMAPHost              string
	IMAPPort              int
	IMAPSecurity          string
	IMAPUsername          string
	EncryptedIMAPPassword []byte
	WrappedDEK            []byte
	KEKVersion            int16
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func EncryptionAAD(userID, accountID string) []byte {
	return []byte(userID + ":" + accountID)
}

func (s *Store) Create(ctx context.Context, account Account) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO mail_accounts (
    id,
    user_id,
    email_address,
    imap_host,
    imap_port,
    imap_security,
    imap_username,
    encrypted_imap_password,
    wrapped_dek,
    kek_version,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW()
)`,
		account.ID,
		account.UserID,
		account.EmailAddress,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPSecurity,
		account.IMAPUsername,
		account.EncryptedIMAPPassword,
		account.WrappedDEK,
		account.KEKVersion,
	)
	if err != nil {
		if isUniqueViolation(err, "uk_mail_accounts_user_id_email_address") {
			return ErrDuplicateAccount
		}
		return fmt.Errorf("insert mail account: %w", err)
	}
	return nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

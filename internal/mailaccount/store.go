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
	SMTPHost              string
	SMTPPort              int
	SMTPSecurity          string
	SMTPUsername          string
	EncryptedSMTPPassword []byte
	WrappedDEK            []byte
	KEKVersion            int16
}

type Summary struct {
	ID           string
	EmailAddress string
}

type Detail struct {
	ID                    string
	EmailAddress          string
	IMAPHost              string
	IMAPPort              int
	IMAPSecurity          string
	IMAPUsername          string
	EncryptedIMAPPassword []byte
	SMTPHost              string
	SMTPPort              int
	SMTPSecurity          string
	SMTPUsername          string
	EncryptedSMTPPassword []byte
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

func (s *Store) ExistsByUserAndEmail(ctx context.Context, userID, emailAddress string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
FROM mail_accounts
WHERE user_id = $1 AND email_address = $2
)`,
		userID,
		emailAddress,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check mail account existence: %w", err)
	}
	return exists, nil
}

func (s *Store) ListSummaries(ctx context.Context, userID string) ([]Summary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, email_address
FROM mail_accounts
WHERE user_id = $1
ORDER BY email_address`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list mail accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Summary
	for rows.Next() {
		var account Summary
		if err := rows.Scan(&account.ID, &account.EmailAddress); err != nil {
			return nil, fmt.Errorf("scan mail account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mail accounts: %w", err)
	}
	return accounts, nil
}

func (s *Store) Get(ctx context.Context, userID, id string) (Detail, bool, error) {
	var account Detail
	err := s.db.QueryRowContext(ctx, `
SELECT id,
       email_address,
       imap_host,
       imap_port,
       imap_security,
       imap_username,
       encrypted_imap_password,
       smtp_host,
       smtp_port,
       smtp_security,
       smtp_username,
       encrypted_smtp_password,
       wrapped_dek,
       kek_version
FROM mail_accounts
WHERE user_id = $1
  AND id = $2`,
		userID,
		id,
	).Scan(
		&account.ID,
		&account.EmailAddress,
		&account.IMAPHost,
		&account.IMAPPort,
		&account.IMAPSecurity,
		&account.IMAPUsername,
		&account.EncryptedIMAPPassword,
		&account.SMTPHost,
		&account.SMTPPort,
		&account.SMTPSecurity,
		&account.SMTPUsername,
		&account.EncryptedSMTPPassword,
		&account.WrappedDEK,
		&account.KEKVersion,
	)
	if err == nil {
		return account, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Detail{}, false, nil
	}
	return Detail{}, false, fmt.Errorf("get mail account: %w", err)
}

func (s *Store) Insert(ctx context.Context, account Account) error {
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
    smtp_host,
    smtp_port,
    smtp_security,
    smtp_username,
    encrypted_smtp_password,
    wrapped_dek,
    kek_version,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW()
)`,
		account.ID,
		account.UserID,
		account.EmailAddress,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPSecurity,
		account.IMAPUsername,
		account.EncryptedIMAPPassword,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPSecurity,
		account.SMTPUsername,
		account.EncryptedSMTPPassword,
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

func (s *Store) Update(ctx context.Context, account Account) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE mail_accounts
SET imap_host = $3,
    imap_port = $4,
    imap_security = $5,
    imap_username = $6,
    encrypted_imap_password = $7,
    smtp_host = $8,
    smtp_port = $9,
    smtp_security = $10,
    smtp_username = $11,
    encrypted_smtp_password = $12,
    updated_at = NOW()
WHERE user_id = $1
  AND id = $2`,
		account.UserID,
		account.ID,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPSecurity,
		account.IMAPUsername,
		account.EncryptedIMAPPassword,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPSecurity,
		account.SMTPUsername,
		account.EncryptedSMTPPassword,
	)
	if err != nil {
		return fmt.Errorf("update mail account: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, userID, id string) error {
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM mail_accounts
WHERE user_id = $1
  AND id = $2`,
		userID,
		id,
	); err != nil {
		return fmt.Errorf("delete mail account: %w", err)
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

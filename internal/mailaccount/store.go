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
	HasIMAP               bool
	IMAPHost              string
	IMAPPort              int
	IMAPSecurity          string
	IMAPUsername          string
	EncryptedIMAPPassword []byte
	HasSMTP               bool
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
	HasIMAP               bool
	IMAPHost              string
	IMAPPort              int
	IMAPSecurity          string
	IMAPUsername          string
	EncryptedIMAPPassword []byte
	HasSMTP               bool
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
	var imapHost, imapSecurity, imapUsername sql.NullString
	var imapPort sql.NullInt64
	var smtpHost, smtpSecurity, smtpUsername sql.NullString
	var smtpPort sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT ma.id,
       ma.email_address,
       ia.host,
       ia.port,
       ia.security,
       ia.username,
       ia.encrypted_password,
       sa.host,
       sa.port,
       sa.security,
       sa.username,
       sa.encrypted_password,
       ma.wrapped_dek,
       ma.kek_version
FROM mail_accounts ma
LEFT JOIN imap_accounts ia ON ia.mail_account_id = ma.id
LEFT JOIN smtp_accounts sa ON sa.mail_account_id = ma.id
WHERE ma.user_id = $1
  AND ma.id = $2`,
		userID,
		id,
	).Scan(
		&account.ID,
		&account.EmailAddress,
		&imapHost,
		&imapPort,
		&imapSecurity,
		&imapUsername,
		&account.EncryptedIMAPPassword,
		&smtpHost,
		&smtpPort,
		&smtpSecurity,
		&smtpUsername,
		&account.EncryptedSMTPPassword,
		&account.WrappedDEK,
		&account.KEKVersion,
	)
	if err == nil {
		account.HasIMAP = imapHost.Valid
		account.IMAPHost = imapHost.String
		if imapPort.Valid {
			account.IMAPPort = int(imapPort.Int64)
		}
		account.IMAPSecurity = imapSecurity.String
		account.IMAPUsername = imapUsername.String
		account.HasSMTP = smtpHost.Valid
		account.SMTPHost = smtpHost.String
		if smtpPort.Valid {
			account.SMTPPort = int(smtpPort.Int64)
		}
		account.SMTPSecurity = smtpSecurity.String
		account.SMTPUsername = smtpUsername.String
		return account, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Detail{}, false, nil
	}
	return Detail{}, false, fmt.Errorf("get mail account: %w", err)
}

func (s *Store) Insert(ctx context.Context, account Account) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mail account insert: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
INSERT INTO mail_accounts (
    id,
    user_id,
    email_address,
    wrapped_dek,
    kek_version,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, NOW(), NOW()
)`,
		account.ID,
		account.UserID,
		account.EmailAddress,
		account.WrappedDEK,
		account.KEKVersion,
	)
	if err != nil {
		if isUniqueViolation(err, "uk_mail_accounts_user_id_email_address") {
			return ErrDuplicateAccount
		}
		return fmt.Errorf("insert mail account: %w", err)
	}
	if account.HasIMAP {
		if err := insertIMAP(ctx, tx, account); err != nil {
			return err
		}
	}
	if account.HasSMTP {
		if err := insertSMTP(ctx, tx, account); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mail account insert: %w", err)
	}
	return nil
}

func (s *Store) Update(ctx context.Context, account Account) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mail account update: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
UPDATE mail_accounts
SET updated_at = NOW()
WHERE user_id = $1
  AND id = $2`,
		account.UserID,
		account.ID,
	)
	if err != nil {
		return fmt.Errorf("update mail account: %w", err)
	}
	if account.HasIMAP {
		if err := upsertIMAP(ctx, tx, account); err != nil {
			return err
		}
	} else if err := deleteIMAP(ctx, tx, account.ID); err != nil {
		return err
	}
	if account.HasSMTP {
		if err := upsertSMTP(ctx, tx, account); err != nil {
			return err
		}
	} else if err := deleteSMTP(ctx, tx, account.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mail account update: %w", err)
	}
	return nil
}

func (s *Store) SaveIMAP(ctx context.Context, account Account) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin imap account save: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := touchMailAccount(ctx, tx, account.UserID, account.ID); err != nil {
		return err
	}
	if err := upsertIMAP(ctx, tx, account); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit imap account save: %w", err)
	}
	return nil
}

func (s *Store) SaveSMTP(ctx context.Context, account Account) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin smtp account save: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := touchMailAccount(ctx, tx, account.UserID, account.ID); err != nil {
		return err
	}
	if err := upsertSMTP(ctx, tx, account); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit smtp account save: %w", err)
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

func (s *Store) DeleteIMAP(ctx context.Context, userID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin imap account delete: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := touchMailAccount(ctx, tx, userID, id); err != nil {
		return err
	}
	if err := deleteIMAP(ctx, tx, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit imap account delete: %w", err)
	}
	return nil
}

func (s *Store) DeleteSMTP(ctx context.Context, userID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin smtp account delete: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := touchMailAccount(ctx, tx, userID, id); err != nil {
		return err
	}
	if err := deleteSMTP(ctx, tx, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit smtp account delete: %w", err)
	}
	return nil
}

type txExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func touchMailAccount(ctx context.Context, tx txExecutor, userID, id string) error {
	result, err := tx.ExecContext(ctx, `
UPDATE mail_accounts
SET updated_at = NOW()
WHERE user_id = $1
  AND id = $2`,
		userID,
		id,
	)
	if err != nil {
		return fmt.Errorf("touch mail account: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read touched mail account count: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func insertIMAP(ctx context.Context, tx txExecutor, account Account) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO imap_accounts (
    mail_account_id,
    host,
    port,
    security,
    username,
    encrypted_password,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, NOW(), NOW()
)`,
		account.ID,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPSecurity,
		account.IMAPUsername,
		account.EncryptedIMAPPassword,
	)
	if err != nil {
		return fmt.Errorf("insert imap account: %w", err)
	}
	return nil
}

func insertSMTP(ctx context.Context, tx txExecutor, account Account) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO smtp_accounts (
    mail_account_id,
    host,
    port,
    security,
    username,
    encrypted_password,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, NOW(), NOW()
)`,
		account.ID,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPSecurity,
		account.SMTPUsername,
		account.EncryptedSMTPPassword,
	)
	if err != nil {
		return fmt.Errorf("insert smtp account: %w", err)
	}
	return nil
}

func upsertIMAP(ctx context.Context, tx txExecutor, account Account) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO imap_accounts (
    mail_account_id,
    host,
    port,
    security,
    username,
    encrypted_password,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, NOW(), NOW()
)
ON CONFLICT (mail_account_id)
DO UPDATE SET
    host = EXCLUDED.host,
    port = EXCLUDED.port,
    security = EXCLUDED.security,
    username = EXCLUDED.username,
    encrypted_password = EXCLUDED.encrypted_password,
    updated_at = NOW()`,
		account.ID,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPSecurity,
		account.IMAPUsername,
		account.EncryptedIMAPPassword,
	)
	if err != nil {
		return fmt.Errorf("upsert imap account: %w", err)
	}
	return nil
}

func upsertSMTP(ctx context.Context, tx txExecutor, account Account) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO smtp_accounts (
    mail_account_id,
    host,
    port,
    security,
    username,
    encrypted_password,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, NOW(), NOW()
)
ON CONFLICT (mail_account_id)
DO UPDATE SET
    host = EXCLUDED.host,
    port = EXCLUDED.port,
    security = EXCLUDED.security,
    username = EXCLUDED.username,
    encrypted_password = EXCLUDED.encrypted_password,
    updated_at = NOW()`,
		account.ID,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPSecurity,
		account.SMTPUsername,
		account.EncryptedSMTPPassword,
	)
	if err != nil {
		return fmt.Errorf("upsert smtp account: %w", err)
	}
	return nil
}

func deleteIMAP(ctx context.Context, tx txExecutor, id string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM imap_accounts WHERE mail_account_id = $1", id); err != nil {
		return fmt.Errorf("delete imap account: %w", err)
	}
	return nil
}

func deleteSMTP(ctx context.Context, tx txExecutor, id string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM smtp_accounts WHERE mail_account_id = $1", id); err != nil {
		return fmt.Errorf("delete smtp account: %w", err)
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

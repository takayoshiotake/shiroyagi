package mailaccount

import (
	"errors"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewIDReturnsUUIDV4(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}

	matched, err := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, id)
	if err != nil {
		t.Fatalf("regexp.MatchString() error = %v", err)
	}
	if !matched {
		t.Fatalf("NewID() = %q, want UUID v4", id)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	err := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "uk_mail_accounts_user_id_email_address",
	}
	if !isUniqueViolation(err, "uk_mail_accounts_user_id_email_address") {
		t.Fatal("isUniqueViolation() = false, want true")
	}

	wrapped := errors.Join(errors.New("insert failed"), err)
	if !isUniqueViolation(wrapped, "uk_mail_accounts_user_id_email_address") {
		t.Fatal("isUniqueViolation(wrapped) = false, want true")
	}
}

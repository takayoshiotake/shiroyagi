package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func TestEnvelopeRoundTripWithMultiplePlaintexts(t *testing.T) {
	kek := bytes.Repeat([]byte{7}, KeySize)
	imapPassword := []byte("imap-password")
	smtpPassword := []byte("smtp-password")
	aad := []byte("user-1:account-1")

	encrypter, err := NewEnvelope(kek, 1, aad)
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	envelope := encrypter.Envelope()
	if envelope.KEKVersion != 1 {
		t.Fatalf("KEKVersion = %d, want 1", envelope.KEKVersion)
	}
	if len(envelope.WrappedDEK) == 0 {
		t.Fatal("WrappedDEK is empty")
	}

	encryptedIMAPPassword, err := encrypter.Encrypt(imapPassword)
	if err != nil {
		t.Fatalf("Encrypt(imapPassword) error = %v", err)
	}
	encryptedSMTPPassword, err := encrypter.Encrypt(smtpPassword)
	if err != nil {
		t.Fatalf("Encrypt(smtpPassword) error = %v", err)
	}
	if bytes.Contains(encryptedIMAPPassword, imapPassword) {
		t.Fatal("encryptedIMAPPassword contains plaintext")
	}
	if bytes.Equal(encryptedIMAPPassword, encryptedSMTPPassword) {
		t.Fatal("ciphertexts are equal")
	}

	decrypter, err := OpenEnvelope(kek, envelope, aad)
	if err != nil {
		t.Fatalf("OpenEnvelope() error = %v", err)
	}
	gotIMAPPassword, err := decrypter.Decrypt(encryptedIMAPPassword)
	if err != nil {
		t.Fatalf("Decrypt(encryptedIMAPPassword) error = %v", err)
	}
	if !bytes.Equal(gotIMAPPassword, imapPassword) {
		t.Fatalf("Decrypt(encryptedIMAPPassword) = %q, want %q", gotIMAPPassword, imapPassword)
	}
	gotSMTPPassword, err := decrypter.Decrypt(encryptedSMTPPassword)
	if err != nil {
		t.Fatalf("Decrypt(encryptedSMTPPassword) error = %v", err)
	}
	if !bytes.Equal(gotSMTPPassword, smtpPassword) {
		t.Fatalf("Decrypt(encryptedSMTPPassword) = %q, want %q", gotSMTPPassword, smtpPassword)
	}
}

func TestEnvelopeRejectsWrongAAD(t *testing.T) {
	kek := bytes.Repeat([]byte{7}, KeySize)
	encrypter, err := NewEnvelope(kek, 1, []byte("user-1:account-1"))
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	_, err = OpenEnvelope(kek, encrypter.Envelope(), []byte("user-2:account-1"))
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("OpenEnvelope() error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestEnvelopeRejectsInvalidKEKSize(t *testing.T) {
	_, err := NewEnvelope([]byte("short"), 1, nil)
	if !errors.Is(err, ErrInvalidKeySize) {
		t.Fatalf("NewEnvelope() error = %v, want ErrInvalidKeySize", err)
	}
}

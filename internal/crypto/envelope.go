package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	BlobVersion byte = 1
	KeySize     int  = 32
	NonceSize   int  = 12
)

var (
	ErrInvalidKeySize     = errors.New("invalid key size")
	ErrInvalidCiphertext  = errors.New("invalid ciphertext")
	ErrUnsupportedVersion = errors.New("unsupported encrypted blob version")
)

type Envelope struct {
	WrappedDEK []byte
	KEKVersion int16
}

type EnvelopeEncrypter struct {
	envelope Envelope
	dek      []byte
}

type EnvelopeDecrypter struct {
	dek []byte
}

func NewEnvelope(kek []byte, kekVersion int16, aad []byte) (*EnvelopeEncrypter, error) {
	if err := validateKey(kek); err != nil {
		return nil, fmt.Errorf("validate KEK: %w", err)
	}

	dek, err := randomBytes(KeySize)
	if err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}

	wrappedDEK, err := encryptAESGCM(kek, dek, aad)
	if err != nil {
		return nil, fmt.Errorf("wrap DEK: %w", err)
	}

	return &EnvelopeEncrypter{
		envelope: Envelope{
			WrappedDEK: wrappedDEK,
			KEKVersion: kekVersion,
		},
		dek: dek,
	}, nil
}

func OpenEnvelope(kek []byte, envelope Envelope, aad []byte) (*EnvelopeDecrypter, error) {
	if err := validateKey(kek); err != nil {
		return nil, fmt.Errorf("validate KEK: %w", err)
	}

	dek, err := decryptAESGCM(kek, envelope.WrappedDEK, aad)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w", err)
	}
	if err := validateKey(dek); err != nil {
		return nil, fmt.Errorf("validate DEK: %w", err)
	}

	return &EnvelopeDecrypter{dek: dek}, nil
}

func (e *EnvelopeEncrypter) Envelope() Envelope {
	return e.envelope
}

func (e *EnvelopeEncrypter) Encrypt(plaintext []byte) ([]byte, error) {
	return e.EncryptWithAAD(plaintext, nil)
}

func (e *EnvelopeEncrypter) EncryptWithAAD(plaintext, aad []byte) ([]byte, error) {
	ciphertext, err := encryptAESGCM(e.dek, plaintext, aad)
	if err != nil {
		return nil, fmt.Errorf("encrypt data: %w", err)
	}
	return ciphertext, nil
}

func (d *EnvelopeDecrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	return d.DecryptWithAAD(ciphertext, nil)
}

func (d *EnvelopeDecrypter) EncryptWithAAD(plaintext, aad []byte) ([]byte, error) {
	ciphertext, err := encryptAESGCM(d.dek, plaintext, aad)
	if err != nil {
		return nil, fmt.Errorf("encrypt data: %w", err)
	}
	return ciphertext, nil
}

func (d *EnvelopeDecrypter) DecryptWithAAD(ciphertext, aad []byte) ([]byte, error) {
	plaintext, err := decryptAESGCM(d.dek, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt data: %w", err)
	}
	return plaintext, nil
}

func encryptAESGCM(key, plaintext, aad []byte) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce, err := randomBytes(NonceSize)
	if err != nil {
		return nil, err
	}

	blob := make([]byte, 1, 1+len(nonce)+len(plaintext)+gcm.Overhead())
	blob[0] = BlobVersion
	blob = append(blob, nonce...)
	blob = gcm.Seal(blob, nonce, plaintext, aad)
	return blob, nil
}

func decryptAESGCM(key, blob, aad []byte) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if len(blob) < 1+NonceSize {
		return nil, ErrInvalidCiphertext
	}
	if blob[0] != BlobVersion {
		return nil, ErrUnsupportedVersion
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := blob[1 : 1+NonceSize]
	ciphertext := blob[1+NonceSize:]
	if len(ciphertext) < gcm.Overhead() {
		return nil, ErrInvalidCiphertext
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}

func validateKey(key []byte) error {
	if len(key) != KeySize {
		return ErrInvalidKeySize
	}
	return nil
}

func randomBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	if _, err := io.ReadFull(cryptorand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

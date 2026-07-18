// Package crypto provides symmetric encryption for secrets at rest
// (Loxone credentials, WebDAV destination configs) using AES-GCM with a key
// derived from the MASTER_KEY environment variable.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// ErrCiphertextTooShort is returned by Decrypt when the input is smaller
// than the AES-GCM nonce size, i.e. it cannot possibly be valid ciphertext
// produced by Encrypt.
var ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")

// DeriveKey derives a 32-byte AES-256 key from the given master key string.
// It is deterministic: the same masterKey always yields the same key.
func DeriveKey(masterKey string) []byte {
	sum := sha256.Sum256([]byte(masterKey))
	return sum[:]
}

// Encrypt encrypts plaintext with AES-256-GCM using a key derived from
// masterKey. The returned bytes are nonce || ciphertext, suitable for
// storing directly in a BLOB column.
func Encrypt(masterKey string, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(DeriveKey(masterKey))
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts data previously produced by Encrypt using the same
// masterKey.
func Decrypt(masterKey string, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(DeriveKey(masterKey))
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrCiphertextTooShort
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return plaintext, nil
}

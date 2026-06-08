package system

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrEncryptionKeyMissing = errors.New("DOKVOL_ENCRYPTION_KEY not set")
	ErrDecryptionFailed     = errors.New("decryption failed")
)

func encryptionKey() ([]byte, error) {
	key := os.Getenv("DOKVOL_ENCRYPTION_KEY")
	if key == "" {
		return nil, ErrEncryptionKeyMissing
	}
	if len(key) < 32 {
		return nil, fmt.Errorf("DOKVOL_ENCRYPTION_KEY must be at least 32 bytes")
	}
	return []byte(key[:32]), nil
}

func EncryptConfig(plaintext string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return plaintext, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func DecryptConfig(encoded string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return encoded, nil
	}

	ciphertext, err := hex.DecodeString(encoded)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrDecryptionFailed
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

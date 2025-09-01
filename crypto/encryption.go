package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateEncryptionKey generates a 256-bit (32 byte) random encryption key
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	return key, nil
}

// EncryptData encrypts data using AES-256-GCM
func EncryptData(data []byte, key []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	return ciphertext, nonce, nil
}

// DecryptData decrypts data using AES-256-GCM
func DecryptData(ciphertext []byte, nonce []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// KeyToBase64 converts a key to base64 string for URL embedding
func KeyToBase64(key []byte) string {
	return base64.URLEncoding.EncodeToString(key)
}

// KeyFromBase64 converts a base64 string back to key bytes
func KeyFromBase64(keyStr string) ([]byte, error) {
	key, err := base64.URLEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key format: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key length: expected 32 bytes, got %d", len(key))
	}
	return key, nil
}
package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate encryption key: %v", err)
	}

	if len(key) != 32 {
		t.Errorf("Expected 32-byte key, got %d bytes", len(key))
	}

	// Generate another key and ensure they're different
	key2, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate second encryption key: %v", err)
	}

	if bytes.Equal(key, key2) {
		t.Error("Two consecutive key generations produced identical keys")
	}
}

func TestEncryptDecryptData(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	testData := []byte("Hello, World! This is terminal data.")
	
	ciphertext, nonce, err := EncryptData(testData, key)
	if err != nil {
		t.Fatalf("Failed to encrypt data: %v", err)
	}

	if bytes.Equal(testData, ciphertext) {
		t.Error("Ciphertext should not equal plaintext")
	}

	decrypted, err := DecryptData(ciphertext, nonce, key)
	if err != nil {
		t.Fatalf("Failed to decrypt data: %v", err)
	}

	if !bytes.Equal(testData, decrypted) {
		t.Errorf("Decrypted data does not match original. Expected: %s, Got: %s", 
			string(testData), string(decrypted))
	}
}

func TestEncryptDecryptEmpty(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	
	testData := []byte("")
	ciphertext, nonce, err := EncryptData(testData, key)
	if err != nil {
		t.Fatalf("Failed to encrypt empty data: %v", err)
	}

	decrypted, err := DecryptData(ciphertext, nonce, key)
	if err != nil {
		t.Fatalf("Failed to decrypt empty data: %v", err)
	}

	if !bytes.Equal(testData, decrypted) {
		t.Error("Decrypted empty data does not match original")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1, _ := GenerateEncryptionKey()
	key2, _ := GenerateEncryptionKey()
	
	testData := []byte("Secret terminal data")
	ciphertext, nonce, err := EncryptData(testData, key1)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Try to decrypt with wrong key
	_, err = DecryptData(ciphertext, nonce, key2)
	if err == nil {
		t.Error("Expected decryption to fail with wrong key, but it succeeded")
	}
}

func TestKeyBase64Encoding(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	
	encoded := KeyToBase64(key)
	if encoded == "" {
		t.Error("Key encoding returned empty string")
	}

	decoded, err := KeyFromBase64(encoded)
	if err != nil {
		t.Fatalf("Failed to decode key: %v", err)
	}

	if !bytes.Equal(key, decoded) {
		t.Error("Decoded key does not match original")
	}
}

func TestKeyFromBase64Invalid(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		shouldErr bool
	}{
		{"invalid base64", "not-valid-base64!", true},
		{"wrong length", base64.URLEncoding.EncodeToString([]byte("short")), true},
		{"empty string", "", true},
		{"valid 32 bytes", base64.URLEncoding.EncodeToString(make([]byte, 32)), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := KeyFromBase64(tc.input)
			if tc.shouldErr && err == nil {
				t.Errorf("Expected error for input '%s', but got none", tc.input)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Expected no error for input '%s', but got: %v", tc.input, err)
			}
		})
	}
}

func TestEncryptionConsistency(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	testData := []byte("Terminal output with special chars: \x1b[31mRED\x1b[0m")

	// Encrypt/decrypt multiple times to ensure consistency
	for i := 0; i < 10; i++ {
		ciphertext, nonce, err := EncryptData(testData, key)
		if err != nil {
			t.Fatalf("Encryption failed on iteration %d: %v", i, err)
		}

		decrypted, err := DecryptData(ciphertext, nonce, key)
		if err != nil {
			t.Fatalf("Decryption failed on iteration %d: %v", i, err)
		}

		if !bytes.Equal(testData, decrypted) {
			t.Errorf("Data mismatch on iteration %d", i)
		}
	}
}

func TestNonceUniqueness(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	testData := []byte("test data")
	
	nonces := make(map[string]bool)
	
	// Generate multiple encryptions and ensure nonces are unique
	for i := 0; i < 100; i++ {
		_, nonce, err := EncryptData(testData, key)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}
		
		nonceStr := base64.StdEncoding.EncodeToString(nonce)
		if nonces[nonceStr] {
			t.Errorf("Duplicate nonce detected: %s", nonceStr)
		}
		nonces[nonceStr] = true
	}
}
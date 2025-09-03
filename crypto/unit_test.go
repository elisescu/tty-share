package crypto

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

// TestEncryptionFunction tests only the EncryptData function
func TestEncryptionFunction(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	testCases := []struct {
		name string
		data []byte
	}{
		{"simple text", []byte("hello world")},
		{"terminal escape codes", []byte("\x1b[31mRED\x1b[0m")},
		{"binary data", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}},
		{"empty data", []byte{}},
		{"large data", bytes.Repeat([]byte("test"), 1000)},
		{"unicode", []byte("🔒 Encrypted terminal session")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, nonce, err := EncryptData(tc.data, key)
			if err != nil {
				t.Fatalf("EncryptData failed: %v", err)
			}

			// Verify ciphertext is different from plaintext (unless empty)
			if len(tc.data) > 0 && bytes.Equal(tc.data, ciphertext) {
				t.Error("Ciphertext should not equal plaintext")
			}

			// Verify nonce is correct size (12 bytes for GCM)
			if len(nonce) != 12 {
				t.Errorf("Expected nonce length 12, got %d", len(nonce))
			}

			// Verify ciphertext includes authentication tag (should be longer than plaintext)
			expectedMinLength := len(tc.data) + 16 // 16-byte auth tag
			if len(ciphertext) < expectedMinLength {
				t.Errorf("Ciphertext too short, expected at least %d bytes, got %d", 
					expectedMinLength, len(ciphertext))
			}
		})
	}
}

// TestDecryptionFunction tests only the DecryptData function
func TestDecryptionFunction(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	testData := []byte("Test decryption with valid data")
	
	// First encrypt the data
	ciphertext, nonce, err := EncryptData(testData, key)
	if err != nil {
		t.Fatalf("Setup encryption failed: %v", err)
	}

	// Test successful decryption
	decrypted, err := DecryptData(ciphertext, nonce, key)
	if err != nil {
		t.Fatalf("DecryptData failed: %v", err)
	}

	if !bytes.Equal(testData, decrypted) {
		t.Errorf("Decrypted data mismatch. Expected: %v, Got: %v", testData, decrypted)
	}

	// Test decryption with wrong key
	wrongKey, _ := GenerateEncryptionKey()
	_, err = DecryptData(ciphertext, nonce, wrongKey)
	if err == nil {
		t.Error("Decryption should fail with wrong key")
	}

	// Test decryption with wrong nonce
	wrongNonce := make([]byte, 12)
	copy(wrongNonce, nonce)
	wrongNonce[0] = wrongNonce[0] ^ 0xFF // Flip bits
	_, err = DecryptData(ciphertext, wrongNonce, key)
	if err == nil {
		t.Error("Decryption should fail with wrong nonce")
	}

	// Test decryption with corrupted ciphertext
	corruptedCiphertext := make([]byte, len(ciphertext))
	copy(corruptedCiphertext, ciphertext)
	corruptedCiphertext[0] = corruptedCiphertext[0] ^ 0xFF // Flip bits
	_, err = DecryptData(corruptedCiphertext, nonce, key)
	if err == nil {
		t.Error("Decryption should fail with corrupted ciphertext")
	}
}

// TestEncryptDecryptRoundTrip tests the complete encrypt->decrypt cycle
func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, _ := GenerateEncryptionKey()

	testCases := [][]byte{
		[]byte("simple test"),
		[]byte(""),
		[]byte("multi\nline\ndata\nwith\nnewlines"),
		[]byte("{\"json\": \"data\", \"test\": true}"),
		bytes.Repeat([]byte("x"), 4096), // Large data
	}

	for i, testData := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			// Encrypt
			ciphertext, nonce, err := EncryptData(testData, key)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}

			// Decrypt
			decrypted, err := DecryptData(ciphertext, nonce, key)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}

			// Verify
			if !bytes.Equal(testData, decrypted) {
				t.Errorf("Round trip failed. Original: %v, Decrypted: %v", testData, decrypted)
			}
		})
	}
}

// TestKeySecurityProperties tests security properties of key generation
func TestKeySecurityProperties(t *testing.T) {
	// Generate multiple keys and ensure they're all different
	keys := make([][]byte, 10)
	for i := range keys {
		key, err := GenerateEncryptionKey()
		if err != nil {
			t.Fatalf("Key generation %d failed: %v", i, err)
		}
		keys[i] = key

		// Check key length
		if len(key) != 32 {
			t.Errorf("Key %d has wrong length: expected 32, got %d", i, len(key))
		}

		// Check for all-zero key (should never happen with good randomness)
		allZero := true
		for _, b := range key {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Errorf("Key %d is all zeros (bad randomness)", i)
		}
	}

	// Ensure all keys are unique
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if bytes.Equal(keys[i], keys[j]) {
				t.Errorf("Keys %d and %d are identical", i, j)
			}
		}
	}
}

// TestEncryptionPerformance tests that encryption doesn't take too long
func TestEncryptionPerformance(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	data := make([]byte, 1024) // 1KB of data
	for i := range data {
		data[i] = byte(i % 256)
	}

	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		_, _, err := EncryptData(data, key)
		if err != nil {
			t.Fatalf("Encryption failed on iteration %d: %v", i, err)
		}
	}

	elapsed := time.Since(start)
	avgPerOp := elapsed / time.Duration(iterations)

	t.Logf("Encrypted %d x 1KB in %v (avg: %v per operation)", iterations, elapsed, avgPerOp)

	// Should be able to encrypt 1KB in under 1ms on average
	if avgPerOp > time.Millisecond {
		t.Errorf("Encryption too slow: %v per 1KB operation", avgPerOp)
	}
}
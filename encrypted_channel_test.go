package main

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/elisescu/tty-share/crypto"
	"github.com/elisescu/tty-share/server"
	"github.com/gorilla/websocket"
)

// Mock PTY handler for testing
type mockPTY struct {
	writtenData []byte
}

func (m *mockPTY) Write(data []byte) (int, error) {
	m.writtenData = append(m.writtenData, data...)
	return len(data), nil
}

func (m *mockPTY) Refresh() {}

func TestEncryptedChannelEndToEnd(t *testing.T) {
	// Generate encryption key
	encryptionKey, err := crypto.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate encryption key: %v", err)
	}

	// Create mock PTY
	mockPty := &mockPTY{}

	// Create server with encryption enabled
	config := server.TTYServerConfig{
		FrontListenAddress: "localhost:0", // Use random port
		FrontendPath:       "",
		PTY:                mockPty,
		SessionID:          "test-session",
		AllowTunneling:     false,
		CrossOrigin:        true,
		BaseUrlPath:        "",
		EncryptionKey:      encryptionKey,
	}

	ttyServer := server.NewTTYServer(config)
	
	// Start test HTTP server
	testServer := httptest.NewServer(ttyServer.GetHandler())
	defer testServer.Close()

	// Test encryption/decryption at protocol level
	testInput := []byte("echo 'Hello Encrypted World!'")
	
	// Test protocol marshaling with encryption  
	serverProto := server.NewTTYProtocolWSLocked(nil, encryptionKey)
	
	writeMsg := server.MsgTTYWrite{
		Data: testInput,
		Size: len(testInput),
	}
	
	// Test server-side encryption (outgoing)
	encryptedData, err := serverProto.MarshalMsg(writeMsg)
	if err != nil {
		t.Fatalf("Failed to marshal encrypted message: %v", err)
	}
	
	// Verify data is encrypted (should contain "Encrypted" type)
	var wrapper server.MsgWrapper
	err = json.Unmarshal(encryptedData, &wrapper)
	if err != nil {
		t.Fatalf("Failed to unmarshal wrapper: %v", err)
	}
	
	if wrapper.Type != "Encrypted" {
		t.Errorf("Expected message type 'Encrypted', got '%s'", wrapper.Type)
	}
	
	// Test client-side decryption (incoming)
	// Simulate receiving the encrypted message
	var encryptedMsg server.MsgEncrypted
	err = json.Unmarshal(wrapper.Data, &encryptedMsg)
	if err != nil {
		t.Fatalf("Failed to unmarshal encrypted message: %v", err)
	}
	
	// Decrypt manually to verify
	decryptedData, err := crypto.DecryptData(encryptedMsg.EncryptedData, encryptedMsg.Nonce, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to decrypt message data: %v", err)
	}
	
	// The decrypted data is the original MsgTTYWrite JSON directly
	var decryptedWriteMsg server.MsgTTYWrite
	err = json.Unmarshal(decryptedData, &decryptedWriteMsg)
	if err != nil {
		t.Fatalf("Failed to parse decrypted write message: %v", err)
	}
	
	if !equalBytes(decryptedWriteMsg.Data, testInput) {
		t.Errorf("Decrypted data doesn't match. Expected: %v, Got: %v", 
			testInput, decryptedWriteMsg.Data)
	}
	
	t.Logf("✅ Successfully tested end-to-end message encryption")
}

func TestEncryptedChannelWithoutKey(t *testing.T) {
	// Generate encryption key for server
	encryptionKey, err := crypto.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate encryption key: %v", err)
	}

	// Create mock PTY
	mockPty := &mockPTY{}

	// Create server with encryption enabled
	config := server.TTYServerConfig{
		FrontListenAddress: "localhost:0",
		PTY:                mockPty,
		SessionID:          "test-session",
		EncryptionKey:      encryptionKey,
	}

	ttyServer := server.NewTTYServer(config)
	testServer := httptest.NewServer(ttyServer.GetHandler())
	defer testServer.Close()

	// Convert HTTP URL to WebSocket URL
	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/s/test-session/ws/"
	wsURL := u.String()

	// Connect as client WITHOUT encryption key
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Create client-side protocol handler without encryption key
	clientProto := server.NewTTYProtocolWSLocked(conn, nil)

	// Send data from server (will be encrypted)
	testOutput := []byte("Encrypted server message")
	_, err = ttyServer.GetSession().Write(testOutput)
	if err != nil {
		t.Fatalf("Failed to write from server: %v", err)
	}

	// Client should receive encrypted data (not decrypted)
	receivedData := make(chan []byte, 1)

	go func() {
		err := clientProto.ReadAndHandle(
			func(data []byte) {
				receivedData <- data
			},
			func(cols, rows int) {
				// Window size changes
			},
			nil, // onEncrypted - not needed for test
		)
		if err != nil {
			t.Logf("Client read error: %v", err)
		}
	}()

	// Wait for data - should be encrypted indicator, not original data
	select {
	case data := <-receivedData:
		dataStr := string(data)
		if !contains(dataStr, "[ENCRYPTED]") {
			t.Errorf("Expected encrypted data indicator, got: %s", dataStr)
		}
		if contains(dataStr, "Encrypted server message") {
			t.Error("Client without key should not see plaintext data")
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for encrypted data indicator")
	}
}

func TestKeyURLGeneration(t *testing.T) {
	key, _ := crypto.GenerateEncryptionKey()
	
	// Test key to base64 encoding
	keyB64 := crypto.KeyToBase64(key)
	if keyB64 == "" {
		t.Error("Key base64 encoding returned empty string")
	}

	// Test key extraction from base64
	decodedKey, err := crypto.KeyFromBase64(keyB64)
	if err != nil {
		t.Fatalf("Failed to decode key from base64: %v", err)
	}

	if !equalBytes(key, decodedKey) {
		t.Error("Decoded key does not match original")
	}

	// Test URL fragment construction
	fragment := "#key=" + keyB64
	
	// Parse URL with fragment
	testURL := "https://example.com/session" + fragment
	u, err := url.Parse(testURL)
	if err != nil {
		t.Fatalf("Failed to parse URL with key fragment: %v", err)
	}

	if u.Fragment != "key="+keyB64 {
		t.Errorf("Fragment not preserved correctly. Expected: key=%s, Got: %s", 
			keyB64, u.Fragment)
	}
}

func TestMessageEncryptionTypes(t *testing.T) {
	key, _ := crypto.GenerateEncryptionKey()

	// Test TTY Write message encryption
	writeMsg := server.MsgTTYWrite{
		Data: []byte("ls -la\n"),
		Size: 6,
	}

	// Test window size message encryption  
	winSizeMsg := server.MsgTTYWinSize{
		Cols: 80,
		Rows: 24,
	}

	// Test that both message types can be encrypted and decrypted
	testCases := []struct {
		name string
		msg  interface{}
	}{
		{"TTYWrite", writeMsg},
		{"TTYWinSize", winSizeMsg},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize message
			msgBytes, err := json.Marshal(tc.msg)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			// Encrypt
			ciphertext, nonce, err := crypto.EncryptData(msgBytes, key)
			if err != nil {
				t.Fatalf("Failed to encrypt message: %v", err)
			}

			// Decrypt
			decrypted, err := crypto.DecryptData(ciphertext, nonce, key)
			if err != nil {
				t.Fatalf("Failed to decrypt message: %v", err)
			}

			// Verify
			if !equalBytes(msgBytes, decrypted) {
				t.Errorf("Decrypted message doesn't match original")
			}
		})
	}
}

// Helper functions
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && 
		   (str == substr || 
			(len(str) > len(substr) && 
			 (str[:len(substr)] == substr || 
			  str[len(str)-len(substr):] == substr || 
			  findInString(str, substr))))
}

func findInString(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
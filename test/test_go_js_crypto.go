package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/elisescu/tty-share/crypto"
)

type CrossLangTestData struct {
	OriginalData  string `json:"originalData"`
	EncryptedData string `json:"encryptedData"` // base64 encoded
	Nonce         string `json:"nonce"`         // base64 encoded  
	Key           string `json:"key"`           // base64 encoded
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <test-data-to-encrypt>\n", os.Args[0])
		os.Exit(1)
	}

	testData := os.Args[1]
	
	// Generate encryption key
	key, err := crypto.GenerateEncryptionKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate key: %v\n", err)
		os.Exit(1)
	}

	// Encrypt the test data
	encrypted, nonce, err := crypto.EncryptData([]byte(testData), key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encrypt data: %v\n", err)
		os.Exit(1)
	}

	// Create JSON output for Node.js to read
	output := CrossLangTestData{
		OriginalData:  testData,
		EncryptedData: base64.StdEncoding.EncodeToString(encrypted),
		Nonce:         base64.StdEncoding.EncodeToString(nonce),
		Key:           base64.StdEncoding.EncodeToString(key),
	}

	// Output JSON to stdout for Node.js to consume
	jsonData, err := json.Marshal(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(jsonData))
}
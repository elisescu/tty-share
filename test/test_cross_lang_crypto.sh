#!/bin/bash

# Cross-language encryption test: Go encrypts, Node.js decrypts

set -e

echo "🧪 Testing cross-language encryption (Go -> Node.js)"

# Test data
TEST_STRING="Hello, encrypted world! 🔒"

# Build Go encryption test program (from parent directory context)
echo "Building Go encryption test..."
cd ../ && go build -o test/go-encrypt-test test/test_go_js_crypto.go && cd test/

# Run Go encryption and pipe to Node.js decryption
echo "Encrypting with Go and decrypting with Node.js..."
echo "Test data: '$TEST_STRING'"
echo

if ./go-encrypt-test "$TEST_STRING" | node test_js_decrypt.js; then
    echo "✅ Cross-language encryption test PASSED"
    exit_code=0
else
    echo "❌ Cross-language encryption test FAILED"
    exit_code=1
fi

# Clean up
rm -f go-encrypt-test

exit $exit_code
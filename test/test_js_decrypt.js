#!/usr/bin/env node

// Cross-language encryption test: Decrypt data encrypted by Go
const crypto = require('crypto');

function base64ToBuffer(base64String) {
    return Buffer.from(base64String, 'base64');
}

function decryptData(encryptedDataWithTag, nonce, key) {
    try {
        // Use the newer crypto API
        const algorithm = 'aes-256-gcm';
        const decipherGCM = crypto.createDecipheriv(algorithm, key, nonce);
        
        // In Go's GCM.Seal(), the auth tag is appended to the ciphertext
        const authTagLength = 16;
        const authTag = encryptedDataWithTag.subarray(-authTagLength);
        const ciphertext = encryptedDataWithTag.subarray(0, -authTagLength);
        
        decipherGCM.setAuthTag(authTag);
        
        let decrypted = decipherGCM.update(ciphertext);
        decrypted = Buffer.concat([decrypted, decipherGCM.final()]);
        
        return decrypted.toString('utf8');
    } catch (error) {
        throw new Error(`Decryption failed: ${error.message}`);
    }
}

// Read JSON from stdin
let inputData = '';
process.stdin.on('data', (chunk) => {
    inputData += chunk;
});

process.stdin.on('end', () => {
    try {
        const testData = JSON.parse(inputData);
        
        // Convert base64 strings back to buffers
        const encryptedData = base64ToBuffer(testData.encryptedData);
        const nonce = base64ToBuffer(testData.nonce);
        const key = base64ToBuffer(testData.key);
        
        // Verify key length
        if (key.length !== 32) {
            console.error(`❌ Invalid key length: expected 32, got ${key.length}`);
            process.exit(1);
        }
        
        // Verify nonce length  
        if (nonce.length !== 12) {
            console.error(`❌ Invalid nonce length: expected 12, got ${nonce.length}`);
            process.exit(1);
        }
        
        console.error(`🔑 Key: ${testData.key.substring(0, 8)}...`);
        console.error(`📊 Encrypted data: ${encryptedData.length} bytes`);
        console.error(`🎲 Nonce: ${nonce.toString('hex')}`);
        
        // Decrypt the data
        const decrypted = decryptData(encryptedData, nonce, key);
        
        // Verify decryption result
        if (decrypted === testData.originalData) {
            console.error('✅ Cross-language decryption successful!');
            console.error(`📝 Original: "${testData.originalData}"`);
            console.error(`📝 Decrypted: "${decrypted}"`);
            process.exit(0);
        } else {
            console.error('❌ Cross-language decryption failed!');
            console.error(`📝 Expected: "${testData.originalData}"`);
            console.error(`📝 Got: "${decrypted}"`);
            process.exit(1);
        }
        
    } catch (error) {
        console.error(`❌ Test failed: ${error.message}`);
        process.exit(1);
    }
});

// Start reading from stdin
process.stdin.resume();
process.stdin.setEncoding('utf8');
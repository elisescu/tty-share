// Browser-side cryptographic utilities for end-to-end encryption

export class CryptoUtils {
    private static async importKey(keyData: Uint8Array): Promise<CryptoKey> {
        return await window.crypto.subtle.importKey(
            "raw",
            keyData,
            { name: "AES-GCM" },
            false,
            ["encrypt", "decrypt"]
        );
    }

    public static async decryptData(
        encryptedData: Uint8Array, 
        nonce: Uint8Array, 
        key: Uint8Array
    ): Promise<Uint8Array> {
        const cryptoKey = await this.importKey(key);
        
        const decryptedBuffer = await window.crypto.subtle.decrypt(
            {
                name: "AES-GCM",
                iv: nonce
            },
            cryptoKey,
            encryptedData
        );
        
        return new Uint8Array(decryptedBuffer);
    }

    public static async encryptData(
        data: Uint8Array,
        key: Uint8Array
    ): Promise<{ encryptedData: Uint8Array; nonce: Uint8Array }> {
        const cryptoKey = await this.importKey(key);
        
        // Generate random nonce
        const nonce = window.crypto.getRandomValues(new Uint8Array(12)); // 96-bit nonce for GCM
        
        const encryptedBuffer = await window.crypto.subtle.encrypt(
            {
                name: "AES-GCM",
                iv: nonce
            },
            cryptoKey,
            data
        );
        
        return {
            encryptedData: new Uint8Array(encryptedBuffer),
            nonce: nonce
        };
    }

    public static keyFromBase64(keyStr: string): Uint8Array | null {
        try {
            // Convert base64 URL-safe encoding to regular base64
            const base64 = keyStr.replace(/-/g, '+').replace(/_/g, '/');
            const binaryString = window.atob(base64);
            const bytes = new Uint8Array(binaryString.length);
            for (let i = 0; i < binaryString.length; i++) {
                bytes[i] = binaryString.charCodeAt(i);
            }
            return bytes.length === 32 ? bytes : null; // Ensure 256-bit key
        } catch (e) {
            return null;
        }
    }

    public static extractKeyFromHash(): Uint8Array | null {
        const hash = window.location.hash;
        if (hash && hash.startsWith('#key=')) {
            const keyStr = hash.substring(5); // Remove '#key='
            return this.keyFromBase64(keyStr);
        }
        return null;
    }
}
# End-to-End Encryption Plan for tty-share

## Overview

Add end-to-end encryption to tty-share so terminal data is encrypted between client and server, preventing proxies and network infrastructure from accessing plaintext data.

**Current State**: TLS transport encryption only - proxies can see all terminal data  
**Goal**: Server generates encryption key, embeds it in URL hash - only clients with full URL can decrypt data

**Key Design**: 
- Server generates encryption key when starting shared shell
- Key embedded in URL hash parameter (e.g., `#key=abc123...`)  
- Hash fragments are not sent to proxy servers (client-side only)
- Without key: users see encrypted terminal data, cannot decrypt

**Example URLs:**
- With key: `https://tty-share.com/s/session123#key=SGVsbG9Xb3JsZA==`
- Without key: `https://tty-share.com/s/session123` (shows encrypted text)

## Implementation Plan

### 1. Add Crypto Dependencies
- Add `golang.org/x/crypto` to Go dependencies
- Add WebCrypto API support for browser clients
- Update build system for new dependencies

### 2. Server Key Generation and URL Construction
- Generate 256-bit encryption key when starting session with `--e2e-encryption`
- Embed key in URL hash fragment: `https://host/session#key=base64encodedkey`
- Display full encrypted URL to user (including hash) on session start
- Store encryption key in server session state for message encryption
- Hash fragment stays client-side, never sent to proxy servers

### 3. Extend Message Protocol  
- Add `MsgIDEncrypted` message type for encrypted terminal data
- Encrypt `MsgTTYWrite.Data` and `MsgTTYWinSize` payloads using AES-256-GCM
- Include nonce/IV in message for each encrypted payload
- Keep existing message types for backward compatibility

### 4. Implement Server-Side Encryption
- Create crypto utilities package for AES-256-GCM operations
- Update `TTYProtocolWSLocked` to encrypt outgoing messages when key present
- Update `marshalMsg()` to handle encrypted message wrapping
- Add `--e2e-encryption` flag to enable encryption mode

### 5. Implement Client-Side Encryption (Go)
- Extract encryption key from URL hash fragment on connection
- Update `ttyShareClient` to decrypt incoming encrypted messages
- Encrypt outgoing user input when key is available
- Gracefully handle missing keys (show encrypted data as-is)

### 6. Implement Client-Side Encryption (Browser)
- Extract key from `window.location.hash` in JavaScript/TypeScript
- Use WebCrypto API for AES-256-GCM decryption
- Update `tty-receiver.ts` to handle encrypted vs unencrypted messages
- Display encryption status indicator in terminal UI
- Show raw encrypted data when key is missing

### 7. Handle Missing Key Scenarios
- Detect when URL hash contains no encryption key
- Display raw encrypted message data directly in terminal (as received)
- Show encryption indicator: "🔒 Encrypted session - key required to decrypt"
- Session remains functional but shows encrypted text instead of terminal output
- Provide clear instructions for obtaining the complete URL with key

### 8. Testing and Validation
- Test key generation and URL construction
- Test encrypted sessions with correct keys
- Test "missing key" scenario shows encrypted data
- Verify proxy servers cannot decrypt (no key access)
- Performance testing with encryption overhead

### 9. Documentation and Release
- Document `--e2e-encryption` flag usage
- Explain URL hash key mechanism  
- Document behavior when key is missing
- Update security documentation

## Key Technical Details

**Encryption**: AES-256-GCM with server-generated key in URL hash  
**Key Distribution**: Server embeds key in URL fragment (`#key=...`)  
**Message Format**: Existing messages wrapped in encrypted envelope  
**Missing Key Behavior**: Show encrypted data as text when hash key missing  
**Proxy Security**: Hash fragments never sent to proxy servers

## Success Criteria

- Terminal data encrypted end-to-end (proxy cannot decrypt)
- Key embedded in URL hash (never sent to proxy servers)
- Sessions without key show encrypted data (not decrypted content)
- Backward compatible with existing unencrypted sessions
- Simple `--e2e-encryption` flag to enable
- <10% performance overhead

## Timeline Estimate

**Total**: 4-6 weeks  
- Steps 1-3 (Foundation): 1-2 weeks
- Steps 4-6 (Implementation): 2-3 weeks  
- Steps 7-9 (Testing & Release): 1 week

*Simplified approach (no key exchange) reduces complexity and timeline*
# End-to-End Encryption Plan for tty-share

## Overview

Add end-to-end encryption to tty-share so terminal data is encrypted between client and server, preventing proxies and network infrastructure from accessing plaintext data.

**Current State**: TLS transport encryption only - proxies can see all terminal data  
**Goal**: Client-to-server encryption where only endpoints can decrypt data

## Implementation Plan

### 1. Add Crypto Dependencies
- Add `golang.org/x/crypto` to Go dependencies
- Add WebCrypto API support for browser clients
- Update build system for new dependencies

### 2. Design Encryption Protocol
- Use AES-256-GCM for message encryption (fast + authenticated)
- Use X25519 ECDH for key exchange (forward secrecy)
- Optional password authentication with PBKDF2

### 3. Extend Message Protocol
- Add `MsgIDKeyExchange` for handshake
- Add `MsgIDEncrypted` for encrypted data
- Modify `MsgWrapper` to include encryption metadata
- Maintain backward compatibility with unencrypted sessions

### 4. Implement Server-Side Encryption
- Create crypto utilities package
- Update `TTYProtocolWSLocked` for encrypted mode
- Add key exchange handshake before terminal communication
- Add `--e2e-encryption` and `--password` flags

### 5. Implement Client-Side Encryption (Go)
- Update `ttyShareClient` to support encryption
- Implement key exchange in client connection flow
- Handle encrypted message sending/receiving
- Auto-detect server encryption capability

### 6. Implement Client-Side Encryption (Browser)
- Add WebCrypto integration to TypeScript client
- Implement browser key exchange
- Update message handling for encryption
- Add encryption status indicator in UI

### 7. Update Proxy Compatibility
- Ensure encrypted sessions work through tty-proxy
- Proxy only routes encrypted traffic (can't decrypt)
- Test with public proxy servers

### 8. Testing and Validation
- Unit tests for crypto functions
- Integration tests for encrypted sessions
- Security testing (key exchange, replay attacks)
- Performance testing (encryption overhead)

### 9. Documentation and Release
- Document new encryption flags and usage
- Update architecture documentation
- Ensure backward compatibility
- Prepare release with security advisory

## Key Technical Details

**Encryption**: AES-256-GCM with X25519 key exchange  
**Message Format**: Wrap existing messages in encrypted envelope  
**Compatibility**: Optional encryption mode, graceful fallback  
**Authentication**: Optional password protection via PBKDF2

## Success Criteria

- Terminal data encrypted end-to-end (proxy cannot decrypt)
- Backward compatible with existing clients  
- Simple `--e2e-encryption` flag to enable
- <10% performance overhead

## Timeline Estimate

**Total**: 6-8 weeks
- Steps 1-3 (Foundation): 2 weeks
- Steps 4-6 (Implementation): 3 weeks  
- Steps 7-9 (Testing & Release): 1-3 weeks
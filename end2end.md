# End-to-End Encryption Implementation Plan for tty-share

## Overview

This document outlines a comprehensive plan to add end-to-end encryption to tty-share, ensuring that terminal data remains encrypted throughout the entire communication path, including when using the proxy server for public sessions.

## Current Architecture Analysis

tty-share operates in two modes:
- **Direct mode**: Local network sharing via direct WebSocket connections
- **Proxy mode**: Public Internet sharing via tty-proxy server

### Current Data Flow
1. PTY Master ↔ Local terminal (stdin/stdout)
2. PTY Master → WebSocket → Remote clients (browser/terminal)
3. WebSocket ← Remote clients → PTY Master (user input)
4. Proxy server (in public mode) forwards all traffic transparently

### Security Gap
Currently, while connections use TLS/HTTPS, the proxy server can decrypt and read all terminal content since it terminates TLS. True end-to-end encryption requires encryption at the application layer.

## Implementation Plan

### 1. Design Encryption Architecture

**1.1** Choose encryption algorithm and key exchange method
- Use **ChaCha20-Poly1305** for symmetric encryption (fast, secure, modern)
- Use **X25519** for Elliptic Curve Diffie-Hellman key exchange
- Use **HKDF-SHA256** for key derivation
- Use **Blake2b** for session authentication

**1.2** Design key exchange flow
- Server generates ephemeral key pair on session creation
- Public key embedded in session URL or exchanged during handshake
- Clients derive shared secret using their ephemeral key pair
- Separate keys for each direction (client→server, server→client)

**1.3** Design encrypted message format
```json
{
  "Type": "EncryptedMessage",
  "Nonce": "base64-encoded-nonce",
  "Ciphertext": "base64-encoded-encrypted-data",
  "OriginalType": "Write|WinSize"
}
```

### 2. Implement Core Cryptographic Components

**2.1** Create cryptographic utilities package
- File: `crypto/e2e_crypto.go`
- Key generation, encryption/decryption functions
- Key derivation and nonce management
- Session key rotation utilities

**2.2** Create session key manager
- File: `crypto/session_keys.go`
- Manage per-session encryption keys
- Handle key rotation and expiration
- Secure key storage and cleanup

**2.3** Add cryptographic dependencies
- Update `go.mod` with required crypto libraries
- Prefer Go standard library (`crypto/*`) where possible
- Add `golang.org/x/crypto` for modern algorithms

### 3. Modify Server-Side Components

**3.1** Update session management (`server/session.go`)
- Add encryption state to `ttyShareSession` struct
- Store session encryption keys securely
- Add methods for encrypted message handling

**3.2** Update protocol layer (`server/tty_protocol_rw.go`)
- Extend `MsgWrapper` to support encrypted messages
- Add `EncryptedMsgWrapper` type
- Modify `marshalMsg()` to encrypt data before sending
- Modify `ReadAndHandle()` to decrypt incoming messages
- Maintain backward compatibility for non-encrypted sessions

**3.3** Update WebSocket handlers (`server/server.go`)
- Add key exchange endpoint: `/s/{session}/keyexchange`
- Modify WebSocket upgrade to include encryption negotiation
- Add session encryption state tracking

### 4. Modify Client-Side Components

**4.1** Update Go client (`client.go`)
- Add encryption capabilities to `ttyShareClient` struct
- Implement key exchange with server
- Encrypt outgoing messages, decrypt incoming messages
- Add command-line flags for encryption control

**4.2** Update browser client (`server/frontend/tty-share/`)
- File: `tty-receiver.ts`
- Add WebCrypto API integration for client-side encryption
- Implement key exchange protocol in JavaScript
- Encrypt messages before sending to WebSocket
- Decrypt received messages before processing

**4.3** Add client crypto utilities
- File: `server/frontend/tty-share/crypto.ts`
- WebCrypto wrapper functions
- Key derivation and management
- Message encryption/decryption

### 5. Implement Key Exchange Protocol

**5.1** Design handshake sequence
1. Client connects to WebSocket endpoint
2. Server sends public key + session parameters
3. Client generates key pair, sends public key
4. Both derive shared secret using ECDH
5. Both derive session keys using HKDF
6. Begin encrypted communication

**5.2** Add key exchange messages
- `KeyExchangeInit`: Server → Client (server public key)
- `KeyExchangeResponse`: Client → Server (client public key)
- `KeyExchangeConfirm`: Server → Client (confirmation + encrypted test)

**5.3** Handle key exchange failures
- Graceful fallback to non-encrypted mode (optional)
- Clear error messages for encryption failures
- Connection timeout handling during key exchange

### 6. Update Message Protocol

**6.1** Extend message types
- Add `MsgIDKeyExchange` constant
- Add `MsgIDEncrypted` for encrypted message wrapper
- Maintain existing `MsgIDWrite` and `MsgIDWinSize` within encrypted payload

**6.2** Implement message encryption
- Encrypt the inner message (`MsgTTYWrite`, `MsgTTYWinSize`) before wrapping
- Use unique nonces for each message
- Add message authentication (AEAD)

**6.3** Handle encryption errors
- Invalid nonce or authentication failures
- Key rotation scenarios
- Connection recovery after encryption failures

### 7. Add Configuration and CLI Options

**7.1** Add server command-line flags
- `--e2e-encryption`: Enable end-to-end encryption (default: false for compatibility)
- `--force-encryption`: Reject non-encrypted connections
- `--key-rotation-interval`: Automatic key rotation period

**7.2** Add client command-line flags
- `--e2e-encryption`: Enable encryption on client side
- `--verify-server`: Verify server identity (optional)

**7.3** Add environment variables
- `TTY_SHARE_E2E_KEY`: Pre-shared key option
- `TTY_SHARE_ENCRYPTION_REQUIRED`: Force encryption requirement

### 8. Update Proxy Integration

**8.1** Modify proxy communication (`proxy/proxy.go`)
- Proxy remains transparent (forwards encrypted data as-is)
- Update hello handshake to include encryption capabilities
- No changes needed to tty-proxy server

**8.2** Test encrypted proxy scenarios
- Verify proxy correctly forwards encrypted WebSocket data
- Test tunnel functionality with encryption enabled
- Ensure proxy logging doesn't capture encrypted content

### 9. Implement Key Management Features

**9.1** Session URL with encryption
- Embed public key or key exchange parameters in session URL
- URL format: `https://server/s/session?pubkey=base64-encoded-key`
- Alternative: QR code generation for easy mobile access

**9.2** Key verification and trust
- Display key fingerprints to users
- Optional key verification step
- Support for trusted key storage

**9.3** Key rotation
- Automatic periodic key rotation
- Re-key on demand
- Graceful handling of key rotation during active sessions

### 10. Testing and Validation

**10.1** Unit tests for crypto components
- Test key generation and exchange
- Test encryption/decryption roundtrip
- Test error handling and edge cases

**10.2** Integration tests
- End-to-end encrypted sessions (direct mode)
- End-to-end encrypted sessions (proxy mode)
- Mixed encrypted/non-encrypted client scenarios
- Browser client encryption functionality

**10.3** Security testing
- Verify no plaintext data in network captures
- Test against key compromise scenarios
- Validate perfect forward secrecy
- Performance impact assessment

### 11. Documentation and User Experience

**11.1** Update documentation
- README.md section on encryption capabilities
- Security documentation in `doc/security.md`
- Architecture updates in `doc/architecture.md`

**11.2** User interface improvements
- Visual indicators for encrypted sessions
- Key fingerprint display in browser UI
- Connection security status

**11.3** Error handling and user guidance
- Clear error messages for encryption failures
- Guidance on troubleshooting encryption issues
- Warning messages for unencrypted sessions

### 12. Backward Compatibility and Migration

**12.1** Maintain compatibility
- Non-encrypted sessions continue to work unchanged
- Gradual encryption adoption path
- Feature detection for mixed-client scenarios

**12.2** Migration utilities
- Detection of encryption-capable clients
- Graceful degradation for legacy clients
- Version negotiation during connection

### 13. Performance Optimization

**13.1** Optimize encryption performance
- Benchmark encryption overhead
- Optimize for high-throughput terminal sessions
- Consider hardware acceleration where available

**13.2** Memory management
- Secure key storage and cleanup
- Efficient message buffering
- Prevent key material leaks

### 14. Security Hardening

**14.1** Implement security best practices
- Constant-time comparisons for key operations
- Secure random number generation
- Key material zeroization after use

**14.2** Add security monitoring
- Log encryption events (without sensitive data)
- Monitor for potential attacks or failures
- Rate limiting for key exchange attempts

## Implementation Priority

### Phase 1: Foundation (Steps 1-3)
- Core cryptographic components
- Basic server-side encryption support
- Message protocol extensions

### Phase 2: Client Integration (Steps 4-6)
- Go client encryption
- Browser client encryption
- Key exchange implementation

### Phase 3: Integration and Testing (Steps 7-10)
- Configuration options
- Proxy integration
- Comprehensive testing

### Phase 4: Polish and Documentation (Steps 11-14)
- User experience improvements
- Documentation updates
- Security hardening

## Security Considerations

1. **Perfect Forward Secrecy**: Each session uses ephemeral keys
2. **Authentication**: Message authentication prevents tampering
3. **Key Rotation**: Regular key rotation limits exposure window
4. **Side-channel Protection**: Constant-time operations where possible
5. **Metadata Protection**: Encrypt message types and sizes where feasible

## Breaking Changes

- New message format for encrypted sessions
- Additional handshake step for key exchange
- Potential performance impact from encryption overhead
- New dependencies in go.mod

## Testing Strategy

1. **Unit Tests**: All crypto functions and message handling
2. **Integration Tests**: Full encrypted session workflows
3. **Security Tests**: Network capture validation, key compromise scenarios
4. **Performance Tests**: Throughput and latency impact measurement
5. **Compatibility Tests**: Mixed encrypted/non-encrypted environments

---

**Note**: This plan prioritizes security and user experience while maintaining backward compatibility. Each step includes specific implementation details and can be tackled incrementally.
# ViSiON/3 BBS - Digital Signature Verification

## Overview

All ViSiON/3 BBS releases are digitally signed with GPG to ensure authenticity and integrity. This guide explains how to verify signatures.

## Quick Verification

### 1. Import the Public Key
```bash
# Download and import the public key
gpg --import vision3-signing-key.asc
```

### 2. Verify a Release File
```bash
# Verify an installer (example for Linux)
gpg --verify vision3-installer-linux-amd64.asc vision3-installer-linux-amd64

# Verify a distribution package
gpg --verify vision3-1.0.0-linux-amd64.tar.gz.asc vision3-1.0.0-linux-amd64.tar.gz

# Verify checksums
gpg --verify SHA256SUMS.asc SHA256SUMS
```

## What Gets Signed

Every release includes signatures for:
- **All installers** (vision3-installer-*.asc)
- **All distribution packages** (vision3-*.tar.gz.asc, vision3-*.zip.asc)  
- **Checksum file** (SHA256SUMS.asc)

## Public Key Information

- **Key ID**: Will be provided after key generation
- **Key Type**: RSA 4096-bit
- **Purpose**: ViSiON/3 BBS Official Release Signing
- **Validity**: Check expiration in key details

## Verification Steps Explained

### Step 1: Import the Public Key
```bash
gpg --import vision3-signing-key.asc
```
This adds the ViSiON/3 signing key to your GPG keyring.

### Step 2: Verify the Key Fingerprint
```bash
gpg --list-keys "ViSiON/3 BBS Project"
```
Compare the fingerprint with the official one published here.

### Step 3: Verify File Signatures
```bash
gpg --verify filename.asc filename
```
- ✅ **Good signature**: File is authentic and unmodified
- ❌ **Bad signature**: File may be corrupted or tampered with
- ⚠️ **Unknown key**: You need to import the public key first

## Troubleshooting

### "Can't check signature: No public key"
- Import the public key: `gpg --import vision3-signing-key.asc`

### "Warning: This key is not certified"
- This is normal for new keys. The signature is still valid.
- You can sign the key locally: `gpg --sign-key "ViSiON/3 BBS Project"`

### "BAD signature"
- **Do not use this file** - it may be corrupted or tampered with
- Re-download from the official source

## Key Rotation

If the signing key changes:
- Old signatures remain valid until key expiration
- New releases will be signed with the new key
- This document will be updated with the new key information

## Security Notes

- Always verify signatures for production deployments
- Signatures protect against accidental corruption and malicious tampering  
- Keep your GPG software updated
- Report any signature verification issues immediately

## For Developers

### Building with Signatures
```bash
# Set your GPG key ID
export GPG_KEY_ID=ABCD1234EFGH5678

# Build with signing enabled (default)
./build-dist.sh

# Build without signing
SIGN_RELEASES=false ./build-dist.sh
```

### Key Requirements
- RSA 4096-bit or higher
- Valid for at least 1 year  
- Associated with project email
- Secure passphrase protection
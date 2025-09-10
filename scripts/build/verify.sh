#!/bin/bash

# ViSiON/3 BBS Release Verification Script
# Automatically verifies digital signatures for downloaded releases

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_header() {
    echo
    echo "=================================================="
    echo "     ViSiON/3 BBS Release Verification Tool"
    echo "=================================================="
    echo
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

check_gpg() {
    if ! command -v gpg >/dev/null 2>&1; then
        print_error "GPG not found. Please install GPG to verify signatures."
        print_info "On Ubuntu/Debian: sudo apt install gnupg"
        print_info "On macOS: brew install gnupg"
        print_info "On CentOS/RHEL: sudo yum install gnupg"
        exit 1
    fi
    print_success "GPG is available"
}

import_public_key() {
    if [ -f "vision3-signing-key.asc" ]; then
        print_info "Importing ViSiON/3 public key..."
        if gpg --import vision3-signing-key.asc 2>/dev/null; then
            print_success "Public key imported successfully"
        else
            print_warning "Key may already be imported"
        fi
    else
        print_error "Public key file 'vision3-signing-key.asc' not found"
        print_info "Make sure you're in the directory with the release files"
        exit 1
    fi
}

verify_file() {
    local file="$1"
    local sig_file="${file}.asc"
    
    if [ ! -f "$file" ]; then
        print_error "File not found: $file"
        return 1
    fi
    
    if [ ! -f "$sig_file" ]; then
        print_warning "Signature not found: $sig_file"
        return 1
    fi
    
    print_info "Verifying: $file"
    if gpg --verify "$sig_file" "$file" 2>/dev/null; then
        print_success "VALID signature: $file"
        return 0
    else
        print_error "INVALID signature: $file"
        return 1
    fi
}

verify_checksums() {
    if [ -f "SHA256SUMS" ] && [ -f "SHA256SUMS.asc" ]; then
        print_info "Verifying checksum file..."
        if verify_file "SHA256SUMS"; then
            print_info "Verifying file checksums..."
            if sha256sum -c SHA256SUMS 2>/dev/null; then
                print_success "All checksums verified"
                return 0
            else
                print_error "Some checksums failed verification"
                return 1
            fi
        else
            print_error "Checksum signature verification failed"
            return 1
        fi
    else
        print_warning "Checksum files not found"
        return 1
    fi
}

auto_detect_files() {
    local files_found=()
    
    # Look for installers
    for installer in vision3-installer-*; do
        if [ -f "$installer" ] && [ "$installer" != "vision3-installer-*" ]; then
            files_found+=("$installer")
        fi
    done
    
    # Look for distribution packages
    for package in vision3-*.tar.gz vision3-*.zip; do
        if [ -f "$package" ] && [[ "$package" != vision3-*.* ]]; then
            files_found+=("$package")
        fi
    done
    
    echo "${files_found[@]}"
}

main() {
    print_header
    
    # Check if GPG is available
    check_gpg
    
    # Import public key
    import_public_key
    
    echo
    print_info "Scanning for ViSiON/3 release files..."
    
    # Auto-detect files to verify
    files=($(auto_detect_files))
    
    if [ ${#files[@]} -eq 0 ]; then
        print_warning "No ViSiON/3 release files found in current directory"
        print_info "This script should be run in the directory containing:"
        print_info "  - vision3-installer-* files"
        print_info "  - vision3-*.tar.gz or vision3-*.zip files"
        print_info "  - vision3-signing-key.asc"
        exit 1
    fi
    
    print_info "Found ${#files[@]} file(s) to verify"
    echo
    
    # Verify each file
    verified=0
    total=${#files[@]}
    
    for file in "${files[@]}"; do
        if verify_file "$file"; then
            ((verified++))
        fi
        echo
    done
    
    # Verify checksums if available
    echo "--- Checksum Verification ---"
    verify_checksums
    echo
    
    # Summary
    echo "=== VERIFICATION SUMMARY ==="
    if [ $verified -eq $total ]; then
        print_success "All $total files verified successfully!"
        print_success "These ViSiON/3 releases are authentic and safe to use."
    elif [ $verified -gt 0 ]; then
        print_warning "$verified of $total files verified successfully"
        print_error "Some files failed verification - DO NOT USE unverified files"
    else
        print_error "No files could be verified!"
        print_error "DO NOT USE these files - they may be corrupted or tampered with"
        exit 1
    fi
    
    echo
    print_info "Verification complete. You can now safely install ViSiON/3 BBS."
}

# Handle command line arguments
case "${1:-}" in
    -h|--help)
        echo "ViSiON/3 BBS Release Verification Tool"
        echo
        echo "Usage: $0 [OPTIONS]"
        echo
        echo "Options:"
        echo "  -h, --help     Show this help message"
        echo "  --key-info     Show public key information"
        echo
        echo "This script automatically:"
        echo "  1. Checks for GPG availability"
        echo "  2. Imports the ViSiON/3 public key"
        echo "  3. Verifies all release files in current directory"
        echo "  4. Validates checksums"
        echo
        echo "Files verified:"
        echo "  - vision3-installer-* (installers)"
        echo "  - vision3-*.tar.gz, vision3-*.zip (packages)"
        echo "  - SHA256SUMS (checksum file)"
        exit 0
        ;;
    --key-info)
        print_header
        if [ -f "vision3-signing-key.asc" ]; then
            print_info "ViSiON/3 Public Key Information:"
            echo
            gpg --import-options show-only --import vision3-signing-key.asc 2>/dev/null || \
                gpg --with-fingerprint vision3-signing-key.asc
        else
            print_error "Public key file 'vision3-signing-key.asc' not found"
        fi
        exit 0
        ;;
    "")
        main
        ;;
    *)
        print_error "Unknown option: $1"
        print_info "Use $0 --help for usage information"
        exit 1
        ;;
esac
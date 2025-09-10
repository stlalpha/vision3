#!/bin/bash

# ViSiON/3 BBS Distribution Builder
# Builds cross-platform binaries and creates distribution packages

set -e

echo "=========================================="
echo "    ViSiON/3 BBS Distribution Builder"
echo "=========================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# GPG signing functions
sign_file() {
    local file="$1"
    if [ "$SIGN_RELEASES" = "true" ] && [ -n "$GPG_KEY_ID" ]; then
        print_status "Signing $file..."
        gpg --armor --detach-sign --default-key "$GPG_KEY_ID" "$file"
        if [ $? -eq 0 ]; then
            print_status "✓ Created signature: ${file}.asc"
        else
            print_warning "Failed to sign $file"
        fi
    fi
}

check_gpg_setup() {
    if [ "$SIGN_RELEASES" = "true" ]; then
        if [ -z "$GPG_KEY_ID" ]; then
            print_warning "GPG_KEY_ID not set. Set it with: export GPG_KEY_ID=your_key_id"
            print_warning "Signatures will be skipped."
            SIGN_RELEASES="false"
            return 1
        fi
        
        if ! command -v gpg >/dev/null 2>&1; then
            print_warning "GPG not found. Install gpg to enable signing."
            SIGN_RELEASES="false"
            return 1
        fi
        
        if ! gpg --list-secret-keys "$GPG_KEY_ID" >/dev/null 2>&1; then
            print_warning "GPG key $GPG_KEY_ID not found in keyring."
            SIGN_RELEASES="false"
            return 1
        fi
        
        print_status "GPG signing enabled with key: $GPG_KEY_ID"
        return 0
    fi
}

# Version and build info
# Production builds require git tags, non-production builds use branch name
if git describe --exact-match --tags HEAD >/dev/null 2>&1; then
    # We're on a tagged commit - use the tag
    VERSION=$(git describe --exact-match --tags HEAD)
    print_status "Building PRODUCTION release: $VERSION"
    RELEASE_TYPE="production"
else
    # Not on a tag - use branch name for non-production build
    BRANCH_NAME=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    SHORT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    VERSION="${BRANCH_NAME}-${SHORT_HASH}"
    print_status "Building NON-PRODUCTION release: $VERSION"
    RELEASE_TYPE="non-production"
fi
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# GPG signing configuration
GPG_KEY_ID="${GPG_KEY_ID:-}"  # Set via environment variable
SIGN_RELEASES="${SIGN_RELEASES:-true}"  # Set to false to disable signing

# Build flags
LDFLAGS="-s -w -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}"

# Check GPG setup
check_gpg_setup

# Create version-specific dist directory (relative to project root)
SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$SCRIPT_DIR/../.."
DIST_DIR="$PROJECT_ROOT/dist/${VERSION}"
rm -rf $DIST_DIR
mkdir -p $DIST_DIR

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

print_step "Building installers for all platforms..."

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PARTS <<< "$platform"
    GOOS=${PARTS[0]}
    GOARCH=${PARTS[1]}
    
    OUTPUT_NAME="vision3-installer-${GOOS}-${GOARCH}"
    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    print_status "Building installer for ${GOOS}/${GOARCH}..."
    
    cd "$(dirname "$0")/../../cmd/install"
    env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="$LDFLAGS" -o "${DIST_DIR}/${OUTPUT_NAME}"
    cd "$(dirname "$0")"
    
    # Sign the installer
    sign_file "${DIST_DIR}/${OUTPUT_NAME}"
done

print_step "Building main application binaries..."

# Build all main applications for each platform
APPS=(
    "vision3"
    "vision3-config" 
    "ansitest"
    "stringtool"
)

# Create distribution README (will be customized per platform)
print_step "Creating platform-specific installation guides..."

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PARTS <<< "$platform"
    GOOS=${PARTS[0]}
    GOARCH=${PARTS[1]}
    
    PLATFORM_DIR="${DIST_DIR}/vision3-${VERSION}-${GOOS}-${GOARCH}"
    mkdir -p "${PLATFORM_DIR}/bin"
    
    print_status "Building applications for ${GOOS}/${GOARCH}..."
    
    for app in "${APPS[@]}"; do
        OUTPUT_NAME="$app"
        if [ $GOOS = "windows" ]; then
            OUTPUT_NAME="${app}.exe"
        fi
        
        cd "$(dirname "$0")/../../cmd/$app"
        env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="$LDFLAGS" -o "${PLATFORM_DIR}/bin/${OUTPUT_NAME}"
        cd "$(dirname "$0")"
    done
    
    # Copy configuration files and assets
    print_status "Copying assets for ${GOOS}/${GOARCH}..."
    cp -r "$PROJECT_ROOT/configs" "${PLATFORM_DIR}/"
    cp -r "$PROJECT_ROOT/menus" "${PLATFORM_DIR}/"
    cp -r "$PROJECT_ROOT/data" "${PLATFORM_DIR}/"
    cp "$PROJECT_ROOT/README.md" "${PLATFORM_DIR}/"
    cp "$PROJECT_ROOT/LICENSE" "${PLATFORM_DIR}/"
    cp "$PROJECT_ROOT/ViSiON3.png" "${PLATFORM_DIR}/"
    
    # Create platform-specific installation guide
    if [ $GOOS = "windows" ]; then
        cat > "${PLATFORM_DIR}/INSTALL.txt" << EOF
ViSiON/3 BBS for Windows ${GOARCH} v${VERSION}
Built on: ${BUILD_DATE}
Git Commit: ${GIT_COMMIT}

=== INSTALLATION INSTRUCTIONS ===

You have already extracted the ViSiON/3 BBS distribution!

QUICK START:
1. Double-click 'start-vision3.bat' to start the BBS server
   (or run 'bin\\vision3.exe' from command prompt)

2. Connect via SSH:
   - Host: localhost
   - Port: 2222
   - Username: felonius
   - Password: password

DETAILED SETUP:
1. Generate SSH host keys (required for first run):
   - Open Command Prompt in this directory
   - Run: bin\\vision3.exe --generate-keys

2. Optional - Configure your BBS:
   - Run: bin\\vision3-config.exe
   - Use arrow keys to navigate menus
   - Edit system name, welcome messages, etc.

3. Start the BBS server:
   - Double-click: start-vision3.bat
   - Or from command line: bin\\vision3.exe

4. Connect with any SSH client:
   - PuTTY, Windows Terminal, or built-in ssh client
   - ssh felonius@localhost -p 2222

IMPORTANT SECURITY:
- Change the default password immediately after first login!
- The default user 'felonius' has sysop privileges

INCLUDED TOOLS:
- bin\\vision3.exe - Main BBS server
- bin\\vision3-config.exe - Configuration utility with arrow navigation
- bin\\ansitest.exe - ANSI art testing tool
- bin\\stringtool.exe - String manipulation utility

SIGNATURE VERIFICATION (Optional but Recommended):
This release is digitally signed. To verify authenticity:
1. Import public key: gpg --import vision3-signing-key.asc
2. Verify signature: gpg --verify vision3-${VERSION}-${GOOS}-${GOARCH}.zip.asc vision3-${VERSION}-${GOOS}-${GOARCH}.zip

For support: https://github.com/stlalpha/vision3

Welcome to the ViSiON/3 BBS experience!
EOF
    else
        # Linux/macOS
        PLATFORM_NAME="Linux"
        if [ $GOOS = "darwin" ]; then
            PLATFORM_NAME="macOS"
        fi
        
        cat > "${PLATFORM_DIR}/INSTALL.txt" << EOF
ViSiON/3 BBS for ${PLATFORM_NAME} ${GOARCH} v${VERSION}
Built on: ${BUILD_DATE}
Git Commit: ${GIT_COMMIT}

=== INSTALLATION INSTRUCTIONS ===

You have already extracted the ViSiON/3 BBS distribution!

QUICK START:
1. Run the startup script: ./start-vision3.sh
   (or directly: ./bin/vision3)

2. Connect via SSH:
   - Host: localhost
   - Port: 2222
   - Username: felonius
   - Password: password

DETAILED SETUP:
1. Make scripts executable (if needed):
   chmod +x start-vision3.sh
   chmod +x bin/*

2. Generate SSH host keys (required for first run):
   ./bin/vision3 --generate-keys

3. Optional - Configure your BBS:
   ./bin/vision3-config
   Use arrow keys to navigate menus and Tab to navigate forms
   Edit system name, welcome messages, pipe codes, etc.

4. Start the BBS server:
   ./start-vision3.sh
   # Or directly:
   ./bin/vision3

5. Connect with SSH:
   ssh felonius@localhost -p 2222

IMPORTANT SECURITY:
- Change the default password immediately after first login!
- The default user 'felonius' has sysop privileges

INCLUDED TOOLS:
- bin/vision3 - Main BBS server
- bin/vision3-config - Configuration utility with full arrow navigation
- bin/ansitest - ANSI art testing tool  
- bin/stringtool - String manipulation utility

SYSTEM REQUIREMENTS:
- Terminal with SSH client
- For best experience: Terminal supporting ANSI colors and CP437 encoding

SIGNATURE VERIFICATION (Optional but Recommended):
This release is digitally signed. To verify authenticity:
1. Import public key: gpg --import vision3-signing-key.asc
2. Verify signature: gpg --verify vision3-${VERSION}-${GOOS}-${GOARCH}.tar.gz.asc vision3-${VERSION}-${GOOS}-${GOARCH}.tar.gz

For support: https://github.com/stlalpha/vision3

Welcome to the ViSiON/3 BBS experience!
EOF
    fi
    
    # Remove SSH keys from configs (will be generated during install)
    rm -f "${PLATFORM_DIR}/configs/ssh_host_*_key"*
    
    # Create platform-specific start script
    if [ $GOOS = "windows" ]; then
        cat > "${PLATFORM_DIR}/start-vision3.bat" << 'EOF'
@echo off
cd /d "%~dp0"
bin\vision3.exe
pause
EOF
    else
        cat > "${PLATFORM_DIR}/start-vision3.sh" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
./bin/vision3
EOF
        chmod +x "${PLATFORM_DIR}/start-vision3.sh"
    fi
    
    # Create archive
    print_status "Creating archive for ${GOOS}/${GOARCH}..."
    cd $DIST_DIR
    if [ $GOOS = "windows" ]; then
        zip -r "vision3-${VERSION}-${GOOS}-${GOARCH}.zip" "vision3-${VERSION}-${GOOS}-${GOARCH}/" 2>&1 >/dev/null
        # Sign the zip file
        cd ..
        sign_file "${DIST_DIR}/vision3-${VERSION}-${GOOS}-${GOARCH}.zip"
    else
        tar -czf "vision3-${VERSION}-${GOOS}-${GOARCH}.tar.gz" "vision3-${VERSION}-${GOOS}-${GOARCH}/"
        # Sign the tar.gz file
        cd ..
        sign_file "${DIST_DIR}/vision3-${VERSION}-${GOOS}-${GOARCH}.tar.gz"
    fi
done

print_step "Creating checksums..."
cd $DIST_DIR
for i in $(find . -type f -print | grep -v '\.asc$'); 
do
sha256sum $i >> SHA256SUMS
done

# Sign the checksum file
sign_file "SHA256SUMS"

# Create signature manifest if signing is enabled
if [ "$SIGN_RELEASES" = "true" ] && [ -n "$GPG_KEY_ID" ]; then
    print_step "Creating signature manifest..."
    
    cat > "SIGNATURES.txt" << EOF
ViSiON/3 BBS v${VERSION} - Digital Signatures
Generated: ${BUILD_DATE}
GPG Key ID: ${GPG_KEY_ID}

=== SIGNATURE VERIFICATION ===

To verify signatures:
1. Import the public key: gpg --import vision3-signing-key.asc
2. Verify a file: gpg --verify filename.asc filename

=== SIGNED FILES ===

EOF
    
    # List all signature files
    for sig_file in $(find . -name "*.asc" | sort); do
        original_file=$(echo "$sig_file" | sed 's/\.asc$//')
        echo "Signature: $sig_file" >> SIGNATURES.txt
        echo "File:      $original_file" >> SIGNATURES.txt
        echo "" >> SIGNATURES.txt
    done
    
    # Add public key and verification scripts to distribution if they exist
    if [ -f "$PROJECT_ROOT/vision3-signing-key.asc" ]; then
        cp "$PROJECT_ROOT/vision3-signing-key.asc" .
        print_status "Added public key to distribution"
    fi
    
    if [ -f "verify.sh" ]; then
        cp "verify.sh" .
        chmod +x verify.sh
        print_status "Added verification script for Linux/macOS"
    fi
    
    if [ -f "verify.bat" ]; then
        cp "verify.bat" .
        print_status "Added verification script for Windows"
    fi
fi

cd ..

echo
print_status "Distribution build completed!"
echo
echo "Files created in ${DIST_DIR}/:"
ls -la $DIST_DIR/
echo

if [ "$SIGN_RELEASES" = "true" ] && [ -n "$GPG_KEY_ID" ]; then
    echo "=== SIGNATURE SUMMARY ==="
    sig_count=$(find $DIST_DIR -name "*.asc" | wc -l)
    print_status "Created $sig_count digital signatures"
    print_status "GPG Key ID: $GPG_KEY_ID"
    print_status "Public key: $DIST_DIR/vision3-signing-key.asc"
    print_status "Signature manifest: $DIST_DIR/SIGNATURES.txt"
    echo
fi

print_status "Distribution packages are ready for release!"

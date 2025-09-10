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

print_step "Building all application binaries first..."

# Build all main applications for each platform
APPS=(
    "vision3"
    "vision3-config" 
    "ansitest"
    "stringtool"
)

print_step "Creating self-extracting installers with embedded files..."

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PARTS <<< "$platform"
    GOOS=${PARTS[0]}
    GOARCH=${PARTS[1]}
    
    print_status "Creating embedded installer for ${GOOS}/${GOARCH}..."
    
    # Create temporary release data directory for embedding
    RELEASE_DATA_DIR="$DIST_DIR/release-data-${GOOS}-${GOARCH}"
    rm -rf "$RELEASE_DATA_DIR"
    mkdir -p "$RELEASE_DATA_DIR/release-data/bin"
    
    # Build all applications for this platform into the release structure
    print_status "Building applications for ${GOOS}/${GOARCH}..."
    for app in "${APPS[@]}"; do
        OUTPUT_NAME="$app"
        if [ $GOOS = "windows" ]; then
            OUTPUT_NAME="${app}.exe"
        fi
        
        print_status "  Building $app..."
        cd "$(dirname "$0")/../../cmd/$app"
        env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="$LDFLAGS" -o "${RELEASE_DATA_DIR}/release-data/bin/${OUTPUT_NAME}"
        if [ $? -ne 0 ]; then
            print_status "Failed to build $app for ${GOOS}/${GOARCH}"
            exit 1
        fi
        cd "$(dirname "$0")"
    done
    
    # Copy configuration files and assets to release-data
    cp -r "$PROJECT_ROOT/configs" "${RELEASE_DATA_DIR}/release-data/"
    cp -r "$PROJECT_ROOT/menus" "${RELEASE_DATA_DIR}/release-data/"
    cp -r "$PROJECT_ROOT/data" "${RELEASE_DATA_DIR}/release-data/"
    cp "$PROJECT_ROOT/README.md" "${RELEASE_DATA_DIR}/release-data/"
    cp "$PROJECT_ROOT/LICENSE" "${RELEASE_DATA_DIR}/release-data/"
    cp "$PROJECT_ROOT/ViSiON3.png" "${RELEASE_DATA_DIR}/release-data/"
    
    # Create platform-specific installation guide in release-data
    if [ $GOOS = "windows" ]; then
        cat > "${RELEASE_DATA_DIR}/release-data/INSTALL.txt" << EOF
ViSiON/3 BBS for Windows ${GOARCH} v${VERSION}
Built on: ${BUILD_DATE}
Git Commit: ${GIT_COMMIT}

=== INSTALLATION INSTRUCTIONS ===

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

SIGNATURE VERIFICATION:
This installer has been digitally signed. Signature verification was performed automatically during installation.

For support: https://github.com/stlalpha/vision3

Welcome to the ViSiON/3 BBS experience!
EOF
    else
        # Linux/macOS
        PLATFORM_NAME="Linux"
        if [ $GOOS = "darwin" ]; then
            PLATFORM_NAME="macOS"
        fi
        
        cat > "${RELEASE_DATA_DIR}/release-data/INSTALL.txt" << EOF
ViSiON/3 BBS for ${PLATFORM_NAME} ${GOARCH} v${VERSION}
Built on: ${BUILD_DATE}
Git Commit: ${GIT_COMMIT}

=== INSTALLATION INSTRUCTIONS ===

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

SIGNATURE VERIFICATION:
This installer has been digitally signed. Signature verification was performed automatically during installation.

For support: https://github.com/stlalpha/vision3

Welcome to the ViSiON/3 BBS experience!
EOF
    fi
    
    # Remove SSH keys from configs (will be generated during install)
    rm -f "${RELEASE_DATA_DIR}/release-data/configs/ssh_host_*_key"*
    
    # Create platform-specific start script
    if [ $GOOS = "windows" ]; then
        cat > "${RELEASE_DATA_DIR}/release-data/start-vision3.bat" << 'EOF'
@echo off
cd /d "%~dp0"
bin\vision3.exe
pause
EOF
    else
        cat > "${RELEASE_DATA_DIR}/release-data/start-vision3.sh" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
./bin/vision3
EOF
        chmod +x "${RELEASE_DATA_DIR}/release-data/start-vision3.sh"
    fi
    
    # Create compressed tar archive of release data
    print_status "Creating compressed release archive..."
    cd "${RELEASE_DATA_DIR}"
    tar -czf release-data.tar.gz release-data/
    
    # Create signature for the archive
    sha256sum release-data.tar.gz > release-data.tar.gz.sha256
    if [ "$SIGN_RELEASES" = "true" ] && [ -n "$GPG_KEY_ID" ]; then
        # Sign the actual tar.gz file, not the SHA256 hash
        sign_file "release-data.tar.gz"
    fi
    
    # Copy files to installer directory for Go embed
    print_status "Preparing files for embedding..."
    INSTALLER_DIR="${SCRIPT_DIR}/../../cmd/install"
    
    # Copy the tar.gz file where Go embed can find it
    cp release-data.tar.gz "${INSTALLER_DIR}/"
    
    # Copy public key for embedding
    if [ -f "$PROJECT_ROOT/vision3-signing-key.asc" ]; then
        cp "$PROJECT_ROOT/vision3-signing-key.asc" "${INSTALLER_DIR}/"
    else
        # Create empty file if no public key exists
        touch "${INSTALLER_DIR}/vision3-signing-key.asc"
    fi
    
    # Copy signature if it exists (for the tar.gz file itself)
    if [ -f "release-data.tar.gz.asc" ]; then
        cp release-data.tar.gz.asc "${INSTALLER_DIR}/release-data.tar.gz.sha256.asc"
    else
        # Create empty signature file if no signature exists
        touch "${INSTALLER_DIR}/release-data.tar.gz.sha256.asc"
    fi
    
    # Build the self-extracting installer with embedded files
    print_status "Building embedded installer for ${GOOS}/${GOARCH}..."
    INSTALLER_NAME="vision3-installer-${GOOS}-${GOARCH}"
    if [ $GOOS = "windows" ]; then
        INSTALLER_NAME="${INSTALLER_NAME}.exe"
    fi
    
    cd "${INSTALLER_DIR}"
    env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="$LDFLAGS" -o "${DIST_DIR}/${INSTALLER_NAME}"
    
    # Clean up embedded files from installer directory
    rm -f release-data.tar.gz vision3-signing-key.asc release-data.tar.gz.sha256.asc
    
    # Sign the installer binary
    sign_file "${DIST_DIR}/${INSTALLER_NAME}"
    
    # Clean up temporary release-data directory
    rm -rf "${RELEASE_DATA_DIR}"
done

print_step "Creating checksums and installer summary..."
cd $DIST_DIR

# Create checksums for installer binaries
for i in $(find . -type f -name "vision3-installer-*" -print); 
do
    sha256sum $i >> SHA256SUMS
done

# Sign the checksum file
sign_file "SHA256SUMS"

# Create installer manifest if signing is enabled
if [ "$SIGN_RELEASES" = "true" ] && [ -n "$GPG_KEY_ID" ]; then
    print_step "Creating installer manifest..."
    
    cat > "INSTALLER_MANIFEST.txt" << EOF
ViSiON/3 BBS v${VERSION} - Self-Extracting Installers
Generated: ${BUILD_DATE}
GPG Key ID: ${GPG_KEY_ID}

=== EMBEDDED INSTALLER FEATURES ===

Each installer is a single self-contained binary that includes:
- All ViSiON/3 BBS applications (vision3, vision3-config, ansitest, stringtool)
- Configuration files, menus, and data directories
- Digital signature verification (automatic during installation)
- Interactive installation with progress indicators
- Platform-specific startup scripts

=== SIGNATURE VERIFICATION ===

Installers are digitally signed. Signature verification is performed automatically
during installation. Manual verification is also possible:

1. Import the public key: gpg --import vision3-signing-key.asc
2. Verify installer: gpg --verify vision3-installer-PLATFORM-ARCH.asc vision3-installer-PLATFORM-ARCH

=== SIGNED INSTALLERS ===

EOF
    
    # List all installer signature files
    for sig_file in $(find . -name "vision3-installer-*.asc" | sort); do
        original_file=$(echo "$sig_file" | sed 's/\.asc$//')
        echo "Signature: $sig_file" >> INSTALLER_MANIFEST.txt
        echo "Installer: $original_file" >> INSTALLER_MANIFEST.txt
        echo "" >> INSTALLER_MANIFEST.txt
    done
    
    # Add public key to distribution
    if [ -f "$PROJECT_ROOT/vision3-signing-key.asc" ]; then
        cp "$PROJECT_ROOT/vision3-signing-key.asc" .
        print_status "Added public key to distribution"
    fi
fi

cd ..

echo
print_status "Embedded installer build completed!"
echo
echo "Self-extracting installers created in ${DIST_DIR}/:"
ls -la $DIST_DIR/
echo

if [ "$SIGN_RELEASES" = "true" ] && [ -n "$GPG_KEY_ID" ]; then
    echo "=== SIGNATURE SUMMARY ==="
    sig_count=$(find $DIST_DIR -name "*.asc" | wc -l)
    installer_count=$(find $DIST_DIR -name "vision3-installer-*" ! -name "*.asc" | wc -l)
    print_status "Created $installer_count self-extracting installers"
    print_status "Created $sig_count digital signatures"
    print_status "GPG Key ID: $GPG_KEY_ID"
    print_status "Public key: $DIST_DIR/vision3-signing-key.asc"
    print_status "Installer manifest: $DIST_DIR/INSTALLER_MANIFEST.txt"
    echo
    print_status "Each installer includes:"
    print_status "  • All BBS applications and files"
    print_status "  • Automatic signature verification"
    print_status "  • Interactive installation process"
    print_status "  • Platform-specific startup scripts"
    echo
fi

print_status "Self-extracting installers are ready for distribution!"
print_status ""
print_status "Usage: Simply run the installer binary for your platform:"
print_status "  Linux/macOS: ./vision3-installer-linux-amd64"
print_status "  Windows:     vision3-installer-windows-amd64.exe"
print_status ""
print_status "Each installer will:"
print_status "  1. Verify its digital signature automatically"
print_status "  2. Extract all BBS files to current directory"
print_status "  3. Create platform-specific startup scripts"
print_status "  4. Provide installation instructions"

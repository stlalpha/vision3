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

# Version and build info
VERSION="1.0.0"
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS="-s -w -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}"

# Create dist directory
DIST_DIR="dist"
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
    
    cd cmd/install
    env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="$LDFLAGS" -o "../../${DIST_DIR}/${OUTPUT_NAME}"
    cd ../..
done

print_step "Building main application binaries..."

# Build all main applications for each platform
APPS=(
    "vision3"
    "vision3-config" 
    "ansitest"
    "stringtool"
)

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
        
        cd "cmd/$app"
        env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="$LDFLAGS" -o "../../${PLATFORM_DIR}/bin/${OUTPUT_NAME}"
        cd ../..
    done
    
    # Copy configuration files and assets
    print_status "Copying assets for ${GOOS}/${GOARCH}..."
    cp -r configs "${PLATFORM_DIR}/"
    cp -r menus "${PLATFORM_DIR}/"
    cp -r data "${PLATFORM_DIR}/"
    cp README.md "${PLATFORM_DIR}/"
    cp LICENSE "${PLATFORM_DIR}/"
    cp ViSiON3.png "${PLATFORM_DIR}/"
    
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
        zip -r "vision3-${VERSION}-${GOOS}-${GOARCH}.zip" "vision3-${VERSION}-${GOOS}-${GOARCH}/"
    else
        tar -czf "vision3-${VERSION}-${GOOS}-${GOARCH}.tar.gz" "vision3-${VERSION}-${GOOS}-${GOARCH}/"
    fi
    cd ..
done

print_step "Creating checksums..."
cd $DIST_DIR
sha256sum * > SHA256SUMS
cd ..

print_step "Creating distribution README..."
cat > "${DIST_DIR}/README.txt" << EOF
ViSiON/3 BBS Distribution v${VERSION}
Built on: ${BUILD_DATE}
Git Commit: ${GIT_COMMIT}

INSTALLATION:

Method 1 - Use the installer (recommended):
1. Download the appropriate installer for your platform:
   - Linux x64: vision3-installer-linux-amd64
   - Linux ARM64: vision3-installer-linux-arm64  
   - macOS Intel: vision3-installer-darwin-amd64
   - macOS Apple Silicon: vision3-installer-darwin-arm64
   - Windows x64: vision3-installer-windows-amd64.exe
   - Windows ARM64: vision3-installer-windows-arm64.exe

2. Run the installer:
   Linux/macOS: chmod +x vision3-installer-* && ./vision3-installer-*
   Windows: double-click vision3-installer-*.exe

Method 2 - Manual installation:
1. Download and extract the appropriate archive for your platform
2. Copy the contents to your desired installation directory
3. Generate SSH host keys (see README.md in the archive)
4. Run ./bin/vision3 (or bin\vision3.exe on Windows)

Default login:
Username: felonius
Password: password
SSH Port: 2222

IMPORTANT: Change the default password after first login!

For support and source code:
https://github.com/stlalpha/vision3

Enjoy your ViSiON/3 BBS!
EOF

echo
print_status "Distribution build completed!"
echo
echo "Files created in ${DIST_DIR}/:"
ls -la $DIST_DIR/
echo
print_status "Distribution packages are ready for release!"
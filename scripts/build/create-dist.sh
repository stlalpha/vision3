#!/bin/bash

# ViSiON/3 BBS Distribution Creator
# Creates complete distribution packages with installer and all files

set -e

VERSION="1.0.0"
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "=========================================="
echo "  ViSiON/3 BBS Distribution Creator v${VERSION}"
echo "=========================================="

# Clean and create dist
rm -rf dist-release
mkdir -p dist-release

echo "[1/4] Building installers for all platforms..."

PLATFORMS=(
    "linux/amd64"
    "darwin/amd64" 
    "darwin/arm64"
    "windows/amd64"
)

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PARTS <<< "$platform"
    GOOS=${PARTS[0]}
    GOARCH=${PARTS[1]}
    
    OUTPUT_NAME="vision3-installer-${GOOS}-${GOARCH}"
    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo "  Building installer for ${GOOS}/${GOARCH}..."
    cd cmd/install
    env GOOS=$GOOS GOARCH=$GOARCH go build -tags release -o "../../dist-release/${OUTPUT_NAME}"
    cd ../..
done

echo "[2/4] Building application binaries for all platforms..."

APPS=("vision3" "vision3-config" "ansitest" "stringtool")

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PARTS <<< "$platform"
    GOOS=${PARTS[0]}
    GOARCH=${PARTS[1]}
    
    PLATFORM_DIR="dist-release/vision3-${VERSION}-${GOOS}-${GOARCH}"
    mkdir -p "${PLATFORM_DIR}/bin"
    
    echo "  Building apps for ${GOOS}/${GOARCH}..."
    
    for app in "${APPS[@]}"; do
        OUTPUT_NAME="$app"
        if [ $GOOS = "windows" ]; then
            OUTPUT_NAME="${app}.exe"
        fi
        
        cd "cmd/$app"
        env GOOS=$GOOS GOARCH=$GOARCH go build -o "../../${PLATFORM_DIR}/bin/${OUTPUT_NAME}"
        cd ../..
    done
done

echo "[3/4] Packaging assets and configuration files..."

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PARTS <<< "$platform"
    GOOS=${PARTS[0]}
    GOARCH=${PARTS[1]}
    
    PLATFORM_DIR="dist-release/vision3-${VERSION}-${GOOS}-${GOARCH}"
    
    echo "  Copying assets for ${GOOS}/${GOARCH}..."
    
    # Copy all necessary files
    cp -r configs "${PLATFORM_DIR}/"
    cp -r menus "${PLATFORM_DIR}/"
    cp -r data "${PLATFORM_DIR}/"
    cp README.md "${PLATFORM_DIR}/"
    cp LICENSE "${PLATFORM_DIR}/"
    cp ViSiON3.png "${PLATFORM_DIR}/"
    
    # Copy the installer into the archive root
    if [ $GOOS = "windows" ]; then
        cp "dist-release/vision3-installer-${GOOS}-${GOARCH}.exe" "${PLATFORM_DIR}/install.exe"
    else
        cp "dist-release/vision3-installer-${GOOS}-${GOARCH}" "${PLATFORM_DIR}/install"
        chmod +x "${PLATFORM_DIR}/install"
    fi
    
    # Remove SSH keys (will be generated during install)
    rm -f "${PLATFORM_DIR}/configs/ssh_host_*_key"*
    
    # Create start scripts
    if [ $GOOS = "windows" ]; then
        cat > "${PLATFORM_DIR}/start-bbs.bat" << 'EOF'
@echo off
title ViSiON/3 BBS Server
cd /d "%~dp0"
echo Starting ViSiON/3 BBS Server...
bin\vision3.exe
pause
EOF
        cat > "${PLATFORM_DIR}/configure.bat" << 'EOF'
@echo off
title ViSiON/3 Configuration
cd /d "%~dp0"
bin\vision3-config.exe
EOF
    else
        cat > "${PLATFORM_DIR}/start-bbs.sh" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
echo "Starting ViSiON/3 BBS Server..."
./bin/vision3
EOF
        cat > "${PLATFORM_DIR}/configure.sh" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
./bin/vision3-config
EOF
        chmod +x "${PLATFORM_DIR}/start-bbs.sh"
        chmod +x "${PLATFORM_DIR}/configure.sh"
    fi
    
    # Create installation instructions
    cat > "${PLATFORM_DIR}/INSTALL.txt" << EOF
ViSiON/3 BBS Installation Instructions

QUICK START:
1. Run the installer to set up in your preferred location:
   ${GOOS:+$([ "$GOOS" = "windows" ] && echo "install.exe" || echo "./install")}

2. Or manually copy this entire directory to your desired location

3. Generate SSH host keys:
   ssh-keygen -t rsa -f configs/ssh_host_rsa_key -N ""
   ssh-keygen -t ed25519 -f configs/ssh_host_ed25519_key -N ""

4. Start the configuration tool:
   ${GOOS:+$([ "$GOOS" = "windows" ] && echo "configure.bat" || echo "./configure.sh")}

5. Start the BBS:
   ${GOOS:+$([ "$GOOS" = "windows" ] && echo "start-bbs.bat" || echo "./start-bbs.sh")}

6. Connect to your BBS:
   ssh felonius@localhost -p 2222
   Default password: password

IMPORTANT: Change the default password after first login!

For file transfers, install lrzsz package on your system.
EOF
done

echo "[4/4] Creating distribution archives..."

cd dist-release

for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -ra PARTS <<< "$platform"
    GOOS=${PARTS[0]}
    GOARCH=${PARTS[1]}
    
    PLATFORM_DIR="vision3-${VERSION}-${GOOS}-${GOARCH}"
    
    echo "  Creating archive for ${GOOS}/${GOARCH}..."
    
    if [ $GOOS = "windows" ]; then
        zip -r "${PLATFORM_DIR}.zip" "${PLATFORM_DIR}/" > /dev/null
    else
        tar -czf "${PLATFORM_DIR}.tar.gz" "${PLATFORM_DIR}/"
    fi
done

# Create checksums
echo "Creating checksums..."
if command -v sha256sum &> /dev/null; then
    sha256sum *.zip *.tar.gz vision3-installer-* > SHA256SUMS
elif command -v shasum &> /dev/null; then
    shasum -a 256 *.zip *.tar.gz vision3-installer-* > SHA256SUMS
fi

cd ..

echo
echo "=========================================="
echo "           DISTRIBUTION COMPLETE!"
echo "=========================================="
echo
echo "Created distribution files in dist-release/:"
echo
ls -la dist-release/ | grep -E '\.(zip|tar\.gz|exe)$|vision3-installer'
echo
echo "Distribution includes:"
echo "• Standalone installers for each platform"
echo "• Complete packages with all binaries and assets"
echo "• Platform-specific start scripts"
echo "• Installation instructions"
echo
echo "Users can either:"
echo "1. Run the standalone installer: ./vision3-installer-<platform>"
echo "2. Extract the complete package and follow INSTALL.txt"

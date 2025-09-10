#!/bin/bash

# ViSiON/3 BBS Installation Script
# This script sets up ViSiON/3 BBS for first-time use

set -e

echo "=========================================="
echo "     ViSiON/3 BBS Installation Script"
echo "=========================================="
echo

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   print_error "This script should not be run as root"
   exit 1
fi

# Check for Go installation
print_step "Checking Go installation..."
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21+ from https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
print_status "Found Go version: $GO_VERSION"

# Check for required external tools
print_step "Checking external dependencies..."

# Check for ZMODEM tools
MISSING_TOOLS=()
if ! command -v sz &> /dev/null; then
    MISSING_TOOLS+=("sz (ZMODEM send)")
fi
if ! command -v rz &> /dev/null; then
    MISSING_TOOLS+=("rz (ZMODEM receive)")
fi

if [ ${#MISSING_TOOLS[@]} -gt 0 ]; then
    print_warning "Missing optional file transfer tools:"
    for tool in "${MISSING_TOOLS[@]}"; do
        echo "  - $tool"
    done
    echo
    print_status "Install lrzsz package for file transfer support:"
    if command -v brew &> /dev/null; then
        echo "  brew install lrzsz"
    elif command -v apt-get &> /dev/null; then
        echo "  sudo apt-get install lrzsz"
    elif command -v yum &> /dev/null; then
        echo "  sudo yum install lrzsz"
    elif command -v dnf &> /dev/null; then
        echo "  sudo dnf install lrzsz"
    else
        echo "  Install lrzsz package using your system's package manager"
    fi
    echo
fi

# Build executables
print_step "Building ViSiON/3 executables..."

# Clean any existing binaries
rm -f vision3 vision3-config ansitest stringtool vision3-bbsconfig

# Build main BBS server
print_status "Building main BBS server..."
cd cmd/vision3
go build -o ../../vision3
cd ../..

# Build configuration tool
print_status "Building configuration tool..."
cd cmd/vision3-config
go build -o ../../vision3-config
cd ../..

# Build utility tools
print_status "Building utility tools..."
cd cmd/ansitest
go build -o ../../ansitest
cd ../..

cd cmd/stringtool
go build -o ../../stringtool
cd ../..

cd cmd/vision3-bbsconfig
go build -o ../../vision3-bbsconfig
cd ../..

print_status "All executables built successfully"

# Create necessary directories
print_step "Creating directory structure..."
mkdir -p data/users data/files/general data/logs
mkdir -p log
print_status "Directory structure created"

# Generate SSH host keys if they don't exist
print_step "Setting up SSH host keys..."
cd configs

if [ ! -f ssh_host_rsa_key ]; then
    print_status "Generating RSA host key..."
    ssh-keygen -t rsa -f ssh_host_rsa_key -N ""
fi

if [ ! -f ssh_host_ed25519_key ]; then
    print_status "Generating Ed25519 host key..."
    ssh-keygen -t ed25519 -f ssh_host_ed25519_key -N ""
fi

if [ ! -f ssh_host_dsa_key ]; then
    print_status "Generating DSA host key..."
    ssh-keygen -t dsa -f ssh_host_dsa_key -N ""
fi

cd ..
print_status "SSH host keys ready"

# Initialize data files if they don't exist
print_step "Initializing data files..."

if [ ! -f data/users/users.json ]; then
    echo '{}' > data/users/users.json
    print_status "Created users database"
fi

if [ ! -f data/oneliners.json ]; then
    echo '[]' > data/oneliners.json
    print_status "Created oneliners database"
fi

print_status "Data files initialized"

# Set appropriate permissions
print_step "Setting file permissions..."
chmod 600 configs/ssh_host_*_key
chmod 644 configs/ssh_host_*_key.pub
chmod +x vision3 vision3-config ansitest stringtool vision3-bbsconfig
print_status "File permissions set"

echo
echo "=========================================="
print_status "Installation completed successfully!"
echo "=========================================="
echo
echo "Quick Start:"
echo "1. Run the configuration tool to customize your BBS:"
echo "   ${BLUE}./vision3-config${NC}"
echo
echo "2. Start the BBS server:"
echo "   ${BLUE}./vision3${NC}"
echo
echo "3. Connect to your BBS:"
echo "   ${BLUE}ssh felonius@localhost -p 2222${NC}"
echo "   Default password: ${YELLOW}password${NC}"
echo
print_warning "IMPORTANT: Change the default password after first login!"
echo
echo "The BBS will listen on port 2222 by default."
echo "Check configs/config.json to change the port."
echo
echo "For file transfers, make sure lrzsz is installed:"
if command -v brew &> /dev/null; then
    echo "  ${BLUE}brew install lrzsz${NC}"
elif command -v apt-get &> /dev/null; then
    echo "  ${BLUE}sudo apt-get install lrzsz${NC}"
else
    echo "  Install lrzsz using your package manager"
fi
echo
print_status "Enjoy your ViSiON/3 BBS!"
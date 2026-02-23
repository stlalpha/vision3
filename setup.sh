#!/bin/bash

# ViSiON/3 BBS Setup Script

echo "=== ViSiON/3 BBS Setup Script ==="
echo

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track if any prerequisites are missing
MISSING_PREREQS=0

echo "Checking prerequisites..."
echo

# Check for Go
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.24.2"
    
    # Simple version comparison (works for most cases)
    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" = "$REQUIRED_VERSION" ]; then
        echo -e "${GREEN}✓${NC} Go $GO_VERSION (required: $REQUIRED_VERSION or higher)"
    else
        echo -e "${RED}✗${NC} Go $GO_VERSION found, but version $REQUIRED_VERSION or higher is required"
        MISSING_PREREQS=1
    fi
else
    echo -e "${RED}✗${NC} Go is not installed"
    echo "  Install from: https://golang.org/dl/"
    MISSING_PREREQS=1
fi

# Check for Git
if command -v git &> /dev/null; then
    GIT_VERSION=$(git --version | awk '{print $3}')
    echo -e "${GREEN}✓${NC} Git $GIT_VERSION"
else
    echo -e "${RED}✗${NC} Git is not installed"
    echo "  Install: sudo apt install git (Debian/Ubuntu) or brew install git (macOS)"
    MISSING_PREREQS=1
fi

# Check for ssh-keygen
if command -v ssh-keygen &> /dev/null; then
    echo -e "${GREEN}✓${NC} SSH client (ssh-keygen)"
else
    echo -e "${RED}✗${NC} SSH client (ssh-keygen) is not installed"
    echo "  Install: sudo apt install openssh-client (Debian/Ubuntu) or included with macOS"
    MISSING_PREREQS=1
fi


# Copy sexyz.ini to bin/ if not present
if [ -f "templates/configs/sexyz.ini" ] && [ ! -f "bin/sexyz.ini" ]; then
    echo "  Creating bin/sexyz.ini from template..."
    cp templates/configs/sexyz.ini bin/sexyz.ini
fi

# Check for sexyz (required — Synchronet ZModem 8k for file transfers)
if [ -x "bin/sexyz" ]; then
    echo -e "${GREEN}✓${NC} sexyz (Synchronet ZModem 8k) at bin/sexyz"
    if [ -f "bin/sexyz.ini" ]; then
        echo -e "${GREEN}✓${NC} sexyz.ini configuration found"
    else
        echo -e "${YELLOW}!${NC} bin/sexyz.ini not found — sexyz will use defaults"
    fi
else
    echo -e "${YELLOW}!${NC} sexyz not found at bin/sexyz (required for file transfers)"
    echo "  Build from source: https://gitlab.synchro.net/main/sbbs.git"
    echo "  See documentation/file-transfer-protocols.md for build instructions"
    echo "  Place the binary at bin/sexyz and make it executable"
fi

echo

# Exit if prerequisites are missing
if [ $MISSING_PREREQS -eq 1 ]; then
    echo -e "${RED}Error: Missing required prerequisites!${NC}"
    echo "Please install the missing components listed above and run setup.sh again."
    echo
    echo "For detailed installation instructions, see: docs/installation.md"
    exit 1
fi

echo -e "${GREEN}All prerequisites satisfied!${NC}"
echo

# Check if SSH host key exists
if [ ! -f "configs/ssh_host_rsa_key" ]; then
    echo "Generating SSH host key (RSA)..."
    ssh-keygen -t rsa -b 4096 -f configs/ssh_host_rsa_key -N "" -q
    echo "SSH host key generated."
else
    echo "SSH host key already exists."
fi

# Create necessary directories
echo "Creating directory structure..."
mkdir -p data/users
mkdir -p data/files/general
mkdir -p data/logs
mkdir -p data/msgbases
mkdir -p data/ftn/{in,secure_in,temp_in,temp_out,out,dupehist,dloads,dloads/pass}
mkdir -p configs
mkdir -p bin
mkdir -p scripts
echo "Directories created."

# Copy template config files to configs/ if they don't exist
echo "Setting up configuration files..."
for template_file in templates/configs/*.json; do
    if [ -f "$template_file" ]; then
        target_file="configs/$(basename "$template_file")"
        if [ ! -f "$target_file" ]; then
            echo "  Creating $(basename "$target_file") from template..."
            cp "$template_file" "$target_file"
        else
            echo "  $(basename "$target_file") already exists, skipping."
        fi
    fi
done

# Copy template IP list files to configs/ if they don't exist
for template_file in templates/configs/*.txt; do
    if [ -f "$template_file" ]; then
        target_file="configs/$(basename "$template_file")"
        if [ ! -f "$target_file" ]; then
            echo "  Creating $(basename "$target_file") from template..."
            cp "$template_file" "$target_file"
        else
            echo "  $(basename "$target_file") already exists, skipping."
        fi
    fi
done

# Copy dosemu2 config templates if dosemu is installed
if command -v dosemu &> /dev/null; then
    echo "Setting up dosemu2 configuration..."
    mkdir -p "$HOME/.dosemu/drive_c"
    if [ ! -f "$HOME/.dosemu/.dosemurc" ]; then
        echo "  Creating .dosemurc from template..."
        cp templates/configs/dosemurc "$HOME/.dosemu/.dosemurc"
    else
        echo "  .dosemurc already exists, skipping."
    fi
    if [ ! -f "$HOME/.dosemu/drive_c/.dosemurc-nocom" ]; then
        echo "  Creating .dosemurc-nocom from template..."
        cp templates/configs/dosemurc-nocom "$HOME/.dosemu/drive_c/.dosemurc-nocom"
    else
        echo "  .dosemurc-nocom already exists, skipping."
    fi
else
    echo -e "${YELLOW}Note:${NC} dosemu2 not installed — skipping .dosemurc setup (only needed for DOS doors)"
fi

# Create initial data files if they don't exist
if [ ! -f "data/oneliners.json" ]; then
    echo "Creating empty oneliners.json..."
    echo "[]" > data/oneliners.json
fi

if [ ! -f "data/users/users.json" ]; then
    echo "Creating initial users.json with default sysop account..."
    cat > data/users/users.json << 'EOF'
[
  {
    "id": 1,
    "username": "felonius",
    "passwordHash": "$2a$10$4BzeQ5Pgg6GT6ckfLtTJOuInTvQxXRSj0DETBGIL87SYG2hHpXbtO",
    "handle": "Felonius",
    "accessLevel": 255,
    "flags": "",
    "lastLogin": "0001-01-01T00:00:00Z",
    "timesCalled": 0,
    "lastBulletinRead": "0001-01-01T00:00:00Z",
    "realName": "System Operator",
    "phoneNumber": "",
    "createdAt": "2024-01-01T00:00:00Z",
    "validated": true,
    "filePoints": 0,
    "numUploads": 0,
    "timeLimit": 60,
    "privateNote": "",
    "current_msg_conference_id": 1,
    "current_msg_conference_tag": "LOCAL",
    "current_file_conference_id": 1,
    "current_file_conference_tag": "LOCAL",
    "group_location": "",
    "current_message_area_id": 1,
    "current_message_area_tag": "GENERAL",
    "current_file_area_id": 1,
    "current_file_area_tag": "GENERAL",
    "screenWidth": 80,
    "screenHeight": 24
  }
]
EOF
fi

if [ ! -f "data/users/callhistory.json" ]; then
    echo "Creating empty callhistory.json..."
    echo "[]" > data/users/callhistory.json
fi

if [ ! -f "data/users/callnumber.json" ]; then
    echo "Creating callnumber.json..."
    echo "1" > data/users/callnumber.json
fi

# Build binaries
echo
echo "Building ViSiON/3..."
go build -o vision3 ./cmd/vision3
echo "Building helper..."
go build -o helper ./cmd/helper
echo "Building v3mail..."
go build -o v3mail ./cmd/v3mail
echo "Building strings..."
go build -o strings ./cmd/strings
echo "Building ue..."
go build -o ue ./cmd/ue

echo "Initializing JAM bases..."
./v3mail stats --all --config configs --data data > /dev/null

echo
echo "=== Setup Complete ==="
echo
echo "Default login: felonius / password"
echo "IMPORTANT: Change the default password immediately!"
echo
echo "To start the BBS:"
echo "  ./vision3"
echo "  or run ./build.sh to rebuild and then ./vision3 to start."
echo
echo "To connect:"
echo "  ssh user@localhost -p 2222"

#!/bin/bash

# ViSiON/3 BBS Setup Script

echo "=== ViSiON/3 BBS Setup Script ==="
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
mkdir -p data/ftn
mkdir -p configs
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
    "accessLevel": 100,
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
echo "Building jamutil..."
go build -o jamutil ./cmd/jamutil

echo "Initializing JAM bases..."
./jamutil stats --all --config configs --data data > /dev/null

echo
echo "=== Setup Complete ==="
echo
echo "Default login: felonius / password"
echo "IMPORTANT: Change the default password immediately!"
echo
echo "To start the BBS:"
echo "  cd cmd/vision3 && ./vision3"
echo "  or run ./build_and_run.sh to build and start in one step."
echo
echo "To connect:"
echo "  ssh user@localhost -p 2222"

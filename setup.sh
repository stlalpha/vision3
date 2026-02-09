#!/bin/bash

# ViSiON/3 BBS Setup Script

echo "=== ViSiON/3 BBS Setup Script ==="
echo

# Check if SSH host keys exist
if [ ! -f "configs/ssh_host_rsa_key" ]; then
    echo "Generating SSH host keys..."
    cd configs/
    ssh-keygen -t rsa -f ssh_host_rsa_key -N "" -q
    ssh-keygen -t ed25519 -f ssh_host_ed25519_key -N "" -q
    ssh-keygen -t dsa -f ssh_host_dsa_key -N "" -q
    cd ..
    echo "SSH host keys generated."
else
    echo "SSH host keys already exist."
fi

# Create necessary directories
echo "Creating directory structure..."
mkdir -p data/users
mkdir -p data/files/general

# Check if initial data files exist
if [ ! -f "data/oneliners.json" ]; then
    echo "Creating empty oneliners.json..."
    echo "[]" > data/oneliners.json
fi

if [ ! -f "data/message_areas.json" ]; then
    echo "Creating initial message areas..."
    cat > data/message_areas.json << 'EOF'
[
    {
        "id": 1,
        "tag": "GENERAL",
        "name": "General Discussion",
        "description": "General discussion area",
        "acs_read": "",
        "acs_post": "",
        "anonymous_allowed": false,
        "real_names_only": false,
        "created_at": "2024-01-01T00:00:00Z",
        "last_post_at": "2024-01-01T00:00:00Z"
    },
    {
        "id": 2,
        "tag": "SYSOP",
        "name": "SysOp Area",
        "description": "Private sysop discussion",
        "acs_read": "s10",
        "acs_post": "s10",
        "anonymous_allowed": false,
        "real_names_only": true,
        "created_at": "2024-01-01T00:00:00Z",
        "last_post_at": "2024-01-01T00:00:00Z"
    }
]
EOF
fi

# Build the BBS
echo
echo "Building ViSiON/3..."
cd cmd/vision3
go build
cd ../..

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
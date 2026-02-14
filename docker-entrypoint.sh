#!/bin/sh

# Create necessary directories
mkdir -p /vision3/configs
mkdir -p /vision3/data/users
mkdir -p /vision3/data/logs
mkdir -p /vision3/data/msgbases/privmail
mkdir -p /vision3/data/files
mkdir -p /vision3/data/ftn

# Generate SSH host keys if missing
if [ ! -f "/vision3/configs/ssh_host_rsa_key" ]; then
    echo "No RSA host key found, generating one..."
    ssh-keygen -t rsa -f /vision3/configs/ssh_host_rsa_key -N "" -q
fi
if [ ! -f "/vision3/configs/ssh_host_ed25519_key" ]; then
    echo "No ED25519 host key found, generating one..."
    ssh-keygen -t ed25519 -f /vision3/configs/ssh_host_ed25519_key -N "" -q
fi

# Copy template configs if configs are missing
if [ ! -f "/vision3/configs/config.json" ]; then
    echo "Initializing config files from templates..."
    cp -n /vision3/templates/configs/*.json /vision3/configs/ 2>/dev/null || true
fi

exec "$@"

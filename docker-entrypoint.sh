#!/bin/sh

# Create necessary directories
mkdir -p /vision3/configs
mkdir -p /vision3/data/users
mkdir -p /vision3/data/logs
mkdir -p /vision3/data/msgbases/privmail
mkdir -p /vision3/data/files
mkdir -p /vision3/temp
for d in \
    /vision3/data/ftn/in \
    /vision3/data/ftn/secure_in \
    /vision3/data/ftn/temp_in \
    /vision3/data/ftn/temp_out \
    /vision3/data/ftn/out \
    /vision3/data/ftn/dupehist \
    /vision3/data/ftn/dloads \
    /vision3/data/ftn/dloads/pass
do
    mkdir -p "$d"
done

# Create per-node temp directories (used by doors and session temp files)
for i in $(seq 1 10); do
    mkdir -p /vision3/temp/node${i}
done

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

# Ensure sexyz.ini is in bin/ (binary must be provided by user)
mkdir -p /vision3/bin
if [ ! -f "/vision3/bin/sexyz.ini" ] && [ -f "/vision3/templates/configs/sexyz.ini" ]; then
    echo "Copying sexyz.ini to bin/..."
    cp /vision3/templates/configs/sexyz.ini /vision3/bin/sexyz.ini
fi

exec "$@"

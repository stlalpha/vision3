#!/bin/sh

mkdir -p /vision3/configs

if [ ! -f "/vision3/configs/ssh_host_rsa_key" ]; then
    echo "No RSA host key found, generating one..."
    ssh-keygen -t rsa -f /vision3/configs/ssh_host_rsa_key -N "" -q
fi
if [ ! -f "/vision3/configs/ssh_host_ed25519_key" ]; then
    echo "No ED25519 host key found, generating one..."
    ssh-keygen -t ed25519 -f /vision3/configs/ssh_host_ed25519_key -N "" -q
fi

exec "$@"

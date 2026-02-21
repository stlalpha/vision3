#!/bin/bash

# ViSiON/3 BBS Build and Run Script
#
# This script automates the build and launch process for the ViSiON/3 BBS system.
# It performs the following tasks:
#
# 1. First-time detection: Checks if essential setup files exist (SSH keys, user database)
# 2. Auto-setup: Runs setup.sh automatically if this is the first build
# 3. Build: Compiles the Go application from cmd/vision3 to the root directory
# 4. Launch: Starts the BBS server if the build succeeds
#
# Usage: ./build-and-run.sh
# The server will continue running until stopped with Ctrl+C

# Check if this is the first time building (setup needed)
if [ ! -f "configs/ssh_host_rsa_key" ] || [ ! -f "data/users/users.json" ]; then
    echo "=== First-time setup detected ==="
    echo "Running setup.sh first..."
    echo
    ./setup.sh
    if [ $? -ne 0 ]; then
        echo "Setup failed!"
        exit 1
    fi
    echo
fi

echo "=== Building ViSiON/3 BBS ==="
if ! go build -o vision3 ./cmd/vision3; then
    echo "Build failed!"
    exit 1
fi
if ! go build -o helper ./cmd/helper; then
    echo "Build failed!"
    exit 1
fi
if ! go build -o jamutil ./cmd/jamutil; then
    echo "Build failed!"
    exit 1
fi
if ! go build -o strings ./cmd/strings; then
    echo "Build failed (strings editor)!"
    exit 1
fi

echo "Build successful!"
echo "============================="
echo

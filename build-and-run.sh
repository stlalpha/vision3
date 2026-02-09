#!/bin/bash

# ViSiON/3 BBS Build and Run Script

echo "=== Building ViSiON/3 BBS ==="
cd cmd/vision3
go build -o ../../vision3

if [ $? -eq 0 ]; then
    echo "Build successful!"
    echo
    echo "=== Starting ViSiON/3 BBS ==="
    echo
    echo "Press Ctrl+C to stop the server"
    echo "=========================================="
    echo
    cd ../..
    ./vision3
else
    echo "Build failed!"
    exit 1
fi

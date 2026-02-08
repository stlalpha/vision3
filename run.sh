#!/bin/bash

# ViSiON/3 BBS Build and Run Script

echo "=== Building ViSiON/3 BBS ==="
cd cmd/vision3
go build

if [ $? -eq 0 ]; then
    echo "Build successful!"
    echo
    echo "=== Starting ViSiON/3 BBS ==="
    echo "Connect via: ssh felonius@localhost -p 2222"
    echo "Default credentials: felonius / password"
    echo
    echo "Press Ctrl+C to stop the server"
    echo "=========================================="
    echo
    cd ../..
    ./cmd/vision3/vision3
else
    echo "Build failed!"
    exit 1
fi

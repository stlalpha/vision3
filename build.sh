#!/bin/bash

# Simple wrapper to run the build script from the project root
cd "$(dirname "$0")/scripts/build"
exec ./build-dist.sh "$@"
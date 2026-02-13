#!/bin/bash

# --------------------------------------------------------
# Launch a local dosemu2 session for configuring DOS doors.
#
# This starts dosemu2 without COM1 serial redirection,
# giving you a direct DOS command prompt to install and
# configure door programs on the virtual C: drive.
#
# Usage: ./scripts/dos-local.sh [drive_c_path]
#
#   drive_c_path  Optional path to drive_c directory.
#                 Defaults to ~/.dosemu/drive_c
# --------------------------------------------------------

trap '' 2

stty cols 80 rows 25

DRIVE_C="${1:-$HOME/.dosemu/drive_c}"
LOG_DIR="$DRIVE_C/nodes"

mkdir -p "$LOG_DIR"

/usr/bin/dosemu \
    -o "$LOG_DIR/dosemu_local.log" \
    -f "$DRIVE_C/.dosemurc-nocom" \
    2>/dev/null

trap 2

# JAM Echomail Support

This document describes how Vision3 formats and writes Fidonet-style echomail into JAM message bases.

## Overview

Vision3 supports echomail and netmail formatting when writing messages into JAM. The core logic lives in `internal/jam` and is used by the message manager when posting to FTN areas.

Key goals:

- Correct JAM subfields for sender, recipient, subject, and addresses
- Proper FTN kludges and origin/tearline formatting
- Consistent MSGID generation

## Message Types

Message type is derived from the area configuration:

- **Local** - Standard local message base
- **Echomail** - Conference/echo messages
- **Netmail** - Direct network messages

Logic lives in `internal/jam/msgtype.go` and is called by `internal/message/manager.go`.

## What Gets Added for Echomail/Netmail

When writing echomail/netmail with `WriteMessageExt`, Vision3 automatically appends:

- `AREA:` kludge for echomail
- `MSGID` (unique serial per base)
- `PID`/`TID` identifiers
- Tearline (`--- Vision3 ...`)
- Origin line (`* Origin: ... (address)`)
- `SEEN-BY` and `PATH` for echomail

Implementation: `internal/jam/echomail.go`, `internal/jam/format.go`, `internal/jam/msgid.go`.

## Configuration Inputs

These values come from configuration and are applied during message creation:

- **Origin address**: `configs/message_areas.json` (`origin_addr` per area)
- **Network tearline**: `configs/ftn.json` (`tearline` per network)
- **BBS name**: `configs/config.json` (`boardName`)

The message manager passes these into the JAM writer.

## Message ID Serial

MSGIDs are generated using a serial counter stored in the JAM fixed header reserved space. The counter increments per message and persists across restarts.

Implementation: `internal/jam/msgid.go`.

## Reading Messages

Incoming echomail tossed by `jamutil toss` is read from JAM and displayed by the message reader. If an origin address is missing from subfields, Vision3 attempts to extract it from the origin line in the text.

Implementation: `internal/jam/message.go`.

## Tests

Echomail support is covered by tests in `internal/jam/echomail_test.go`.

Run:

```bash
go test ./internal/jam/... -v
```

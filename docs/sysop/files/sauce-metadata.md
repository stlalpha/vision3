# SAUCE Metadata Handling

## Overview

ViSiON/3 automatically strips SAUCE (Standard Architecture for Universal Comment Extensions) metadata from ANSI art files during display. This ensures that metadata doesn't appear as garbage characters on screen.

## What is SAUCE?

SAUCE is a metadata format commonly embedded in ANSI art files. It contains information about the artwork such as:

- Title (35 characters)
- Author (20 characters)
- Group/Organization (20 characters)
- Creation Date (YYYYMMDD format)
- Font information (e.g., "IBM VGA")
- File type and data type information

## Format Structure

A SAUCE record consists of:

1. **EOF Marker** (1 byte): `0x1A` (Ctrl-Z) - marks the end of the actual content
2. **Optional Comment Block**: May contain multiple 64-character comment lines
3. **SAUCE Record** (128 bytes): Starts with the signature "SAUCE00"

## Implementation

The SAUCE stripping is implemented in `internal/ansi/ansi.go`:

- The `GetAnsiFileContent()` function automatically calls `stripSAUCE()` after reading the file
- Detection: Checks if the last 128 bytes start with "SAUCE"
- Removal: Searches backwards for the EOF marker (0x1A) and truncates the content before it
- Fallback: If no EOF marker is found, removes just the 128-byte SAUCE record

## Where SAUCE Stripping is Applied

The following ANSI files automatically have SAUCE metadata stripped:

- **Pre-login screens**: `PDMATRIX.ANS` (pre-login menu)
- **Pre-login splash**: `PRELOGON.ANS` and numbered variants (`PRELOGON.1`, `PRELOGON.2`, etc.)
- **Login screen**: `LOGIN.ANS` (via `ProcessAnsiAndExtractCoords`)
- **Menu displays**: Any menu using `GetAnsiFileContent()` or `ProcessAnsiAndExtractCoords()`

All ANSI files loaded through these functions will have SAUCE stripped automatically, preventing metadata from displaying as garbage characters.

## Testing

Tests are located in `internal/ansi/sauce_test.go` and cover:

- Files without SAUCE metadata (pass through unchanged)
- SAUCE with EOF marker (proper removal)
- SAUCE without EOF marker (fallback removal)
- Malformed SAUCE signatures (ignored)
- Files too small to contain SAUCE (ignored)

Run tests with:

```bash
go test ./internal/ansi -v -run TestStripSAUCE
```

## References

- [SAUCE Specification (Revision 5)](https://www.acid.org/info/sauce/sauce.htm)
- [ANSI Art Format Documentation](http://fileformats.archiveteam.org/wiki/ANSI_Art)

## Example

Before stripping:

```text
[ANSI art content]^Z SAUCE00 [metadata bytes...]
```

After stripping:

```text
[ANSI art content]
```

The EOF marker (^Z / 0x1A) and everything after it is removed.

# VIS-11: Externalize Hardcoded Strings to strings.json

## Overview

Replace all hardcoded user-facing strings across the codebase with configurable entries in `configs/strings.json`, using the original Vision 2 `STRINGS.PAS` values as defaults where applicable.

## Current State

- `StringsConfig` struct in `internal/config/config.go` has ~205 fields
- `configs/strings.json` has values for ~100 of those fields (pages 1-4 of V2)
- Pages 5-9 fields exist in the struct but have no JSON values
- ~180+ hardcoded strings in Go source files bypass `LoadedStrings` entirely

## Source of Truth

Vision 2's `STRINGS.PAS` (`/Users/jm/Projects/vision-2-bbs/SRC/STRINGS.PAS`) defines 178 configurable strings with their default pipe-coded values in the `FormatStrings` procedure. These are the canonical defaults.

## Approach

### 1. Populate missing V2 strings in strings.json

Add JSON entries for all StringsConfig fields that exist in the struct but lack values in `strings.json`. Use the original V2 defaults from `STRINGS.PAS`.

### 2. Add V3-specific fields to StringsConfig

For hardcoded strings that have no V2 equivalent (chat/page messages, system stats labels, user config prompts, admin strings, file operations, error messages), add new fields to `StringsConfig` with sensible defaults.

### 3. Replace hardcoded strings with LoadedStrings references

In each Go source file, replace hardcoded string literals with `e.LoadedStrings.FieldName`. For format strings with `%s`/`%d` verbs, use `fmt.Sprintf(e.LoadedStrings.FieldName, args...)`.

### 4. Fallback pattern

The existing pattern works: if a field has no JSON value, it gets Go's zero value (empty string). Code that uses loaded strings should check for empty and fall back to a hardcoded default. However, since we're populating all values in strings.json, this is a safety net only.

## Files Modified

| File | Change |
|---|---|
| `internal/config/config.go` | Add ~80 new StringsConfig fields for V3-specific strings |
| `configs/strings.json` | Add ~180 entries (V2 defaults + V3 defaults) |
| `templates/configs/strings.json` | Mirror configs/strings.json |
| `internal/menu/executor.go` | Replace ~60 hardcoded strings |
| `internal/menu/chat.go` | Replace ~4 hardcoded strings |
| `internal/menu/page.go` | Replace ~10 hardcoded strings |
| `internal/menu/newuser.go` | Replace hardcoded strings |
| `internal/menu/message_reader.go` | Replace ~10 hardcoded strings |
| `internal/menu/message_scan.go` | Replace ~10 hardcoded strings |
| `internal/menu/message_list.go` | Replace ~11 hardcoded strings |
| `internal/menu/file_viewer.go` | Replace ~3 hardcoded strings |
| `internal/menu/door_handler.go` | Replace ~8 hardcoded strings |
| `internal/menu/user_config.go` | Replace ~20 hardcoded strings |
| `internal/menu/system_stats.go` | Replace ~10 hardcoded strings |
| `internal/menu/matrix.go` | Replace ~2 hardcoded strings |
| `internal/menu/conference_menu.go` | Replace ~4 hardcoded strings |

## Naming Convention

JSON keys use camelCase matching the existing pattern in strings.json. New V3-specific keys follow the pattern: `{category}{Description}` (e.g., `chatUserEntered`, `pageOnlineNodesHeader`, `statsVersion`).

## Not In Scope

- Internationalization/multi-language support (future)
- Runtime string reloading without restart
- Admin UI for editing strings

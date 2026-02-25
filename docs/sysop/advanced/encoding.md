# Encoding Invariants (UTF-8 / CP437)

This project supports two output modes:
- `UTF-8` terminal output
- `CP437` terminal output

The goal is **consistent rendering** for all user-visible text, prompts, ANSI templates, and control sequences.

## Invariants

1. **All terminal/session output must go through `terminalio` helpers**
   - Preferred: `terminalio.WriteProcessedBytes(...)`
   - Use `terminalio.WriteStringCP437(...)` for known UTF-8 source strings when CP437 conversion is required.

2. **Do not call `terminal.Write(...)` directly for user-visible output**
   - Exception: commented code or legacy debug snippets not used at runtime.

3. **Always process pipe codes before display**
   - Use `ansi.ReplacePipeCodes(...)` for strings containing `|xx` codes.

4. **Never hard-force `ansi.OutputModeCP437` on user-facing paths**
   - Use the active session output mode, propagated through function parameters/context.

5. **ANSI/control sequences must preserve semantics across modes**
   - Cursor movement/clear sequences should still be written via `terminalio` so mode handling remains centralized.

## Canonical Path

- `internal/terminalio/writer.go`
  - `WriteProcessedBytes(...)` is the central mode-aware write path.
  - CP437 mode routes through conversion logic.
  - UTF-8 mode converts raw CP437 high bytes to UTF-8 when needed while preserving ANSI escapes.
  - **Rune-by-rune decoding**: Preserves valid UTF-8 while mapping invalid bytes to CP437.
  - Handles mixed encoding content correctly without corruption.

## Audit Commands

Run from repo root:

```bash
grep -RInE 'terminal\.Write\(|fmt\.Fprint\(|session\.Write\(|s\.Write\(' internal
```

```bash
grep -RInE 'WriteProcessedBytes\(.*OutputModeCP437|OutputModeCP437\)' internal/menu
```

```bash
grep -RInE 'WriteProcessedBytes\(.*\[]byte\("\|' internal/menu
```

## Recent Improvements (2026-02-14)

### Mixed UTF-8 + CP437 Handling
- **Fix Applied**: `terminalio.WriteProcessedBytes` now uses rune-by-rune decoding
- **Before**: Entire span treated as CP437 if ANY byte was invalid UTF-8
- **After**: Preserves valid UTF-8 sequences, only maps invalid bytes to CP437
- **Impact**: Correctly handles mixed content (e.g., UTF-8 text with CP437 box drawing)
- **Implementation**: `internal/terminalio/writer.go` lines 31-52

### ANSI Renderer Styled Spaces
- **Fix Applied**: Preserves spaces with non-default styles (background colors)
- **Before**: Trailing space trimming removed styled spaces
- **After**: Checks `cell.Style != "\x1b[0m"` when finding rightmost character
- **Impact**: ANSI art background colors no longer stripped
- **Implementation**: `internal/menu/ansi_renderer.go` lines 365-372

## Current Status (2026-02-14)

- ✅ Menu/runtime output paths are normalized to `terminalio` helpers
- ✅ Mixed UTF-8 + CP437 content handled correctly (rune-by-rune decoding)
- ✅ ANSI renderer preserves styled spaces with background colors
- ⚠️ Template file corruption documented: DOORLIST.BOT, DOORLIST.TOP contain U+FFFD (see `menus/v3/templates/README-ENCODING-ISSUES.md`)
- One remaining `terminal.Write(...)` occurrence is a commented historical line in `internal/menu/executor.go`
- `internal/ansi/ansi.go` still contains direct `session.Write(...)` in ANSI utility/display functions; treat this package as a specialized rendering layer and keep mode behavior explicit there

## Change Checklist (for future PRs)

- [ ] New output code uses `terminalio` helpers only.
- [ ] Any `|xx` string is wrapped with `ansi.ReplacePipeCodes(...)` before write.
- [ ] No forced `OutputModeCP437` in user-facing menu/session flows.
- [ ] Build passes: `go build ./cmd/vision3`.
- [ ] Grep audit commands above show no new bypasses.

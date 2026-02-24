# Bulk File Import

The `helper files` commands provide sysop tools for importing files into BBS file areas in bulk and managing FILE_ID.DIZ descriptions.

## Prerequisites

Build the helper binary:

```bash
go build -o helper ./cmd/helper/
```

Run it from the ViSiON/3 root directory (where `configs/` and `data/` are located), or use the `--config` and `--data` flags to specify paths.

## Commands

### `helper files import` — Bulk Import

Import all files from a source directory into a file area. Automatically extracts FILE_ID.DIZ descriptions from supported archives.

```text
Usage: helper files import [options]

Options:
  --dir DIR          Source directory containing files to import (required)
  --area TAG         Target file area tag, e.g. GENERAL (required)
  --uploader NAME    Uploader handle (default: "Sysop")
  --data DIR         Data directory (default: "data")
  --config DIR       Config directory (default: "configs")
  --move             Move files instead of copying (default: copy)
  --dry-run          Show what would be imported without making changes
  --no-diz           Skip FILE_ID.DIZ extraction from archives
  --preserve-dates   Use file modification time as upload date (default: current time)
```

#### Examples

Preview what would be imported:

```bash
./helper files import --dir /mnt/cdrom/files --area GENERAL --dry-run
```

Import files from a CD-ROM dump with original dates preserved:

```bash
./helper files import --dir /mnt/cdrom/utils --area UTILS --preserve-dates --uploader "Sysop"
```

Import and move files from a staging directory:

```bash
./helper files import --dir /tmp/incoming --area UPLOADS --move
```

#### Import Behavior

**File scanning:**
- Scans the source directory (non-recursive, top level only)
- Skips directories, hidden files (dotfiles), and system files (`files.bbs`, `metadata.json`, `Thumbs.db`, `.DS_Store`, `desktop.ini`)

**Duplicate detection:**
- Compares filenames (case-insensitive) against the target area's existing `metadata.json`
- Duplicate files are skipped with a warning

**DIZ extraction:**
- For supported archive types (determined by `configs/archivers.json`), FILE_ID.DIZ is extracted and used as the file description
- ZIP files are read natively; other formats use their configured external extract commands
- DIZ search is case-insensitive (`FILE_ID.DIZ`, `file_id.diz`, etc.)
- Use `--no-diz` to skip extraction (descriptions will be empty)

**File transfer:**
- Default: files are **copied** to the area directory
- With `--move`: files are moved (renamed; falls back to copy+delete across filesystems)
- Source files are preserved unless `--move` is specified

**Metadata:**
- Each imported file gets a new UUID and `FileRecord` entry
- All new records are appended to the area's `metadata.json` in a single atomic write
- In `--dry-run` mode, no files are copied and no metadata is written

#### Output Format

```text
Import to: General Files (GENERAL)
Area path: data/files/general
Source:    /mnt/cdrom/files
Uploader:  Sysop
Files:     42 candidates
Mode:      COPY

  OK    COOLAPP.ZIP                              510.2 KB +DIZ
  OK    README.TXT                                 2.1 KB
  SKIP  EXISTING.ZIP                             (duplicate)
  ERR   BROKEN.ZIP                               permission denied

Summary: 40 imported, 1 skipped, 1 errors
```

Status codes:
- `OK` — file imported successfully (`+DIZ` if description was extracted)
- `SKIP` — file already exists in the area
- `ERR` — import failed (error shown)
- `WARN` — non-fatal issue (e.g., DIZ extraction failed but file still imported)
- `ADD` — would be imported (dry run only)

### `helper files reextractdiz` — Re-extract DIZ

Re-scan existing files in one or all areas and update descriptions from FILE_ID.DIZ content found in archives. Useful after a bulk import with `--no-diz`, or when DIZ extraction has been improved.

```text
Usage: helper files reextractdiz [options]

Options:
  --area TAG     Specific file area tag (omit for all areas)
  --data DIR     Data directory (default: "data")
  --config DIR   Config directory (default: "configs")
  --dry-run      Show what would be updated without making changes
```

#### Examples

Re-extract DIZ for a specific area:

```bash
./helper files reextractdiz --area GENERAL
```

Preview changes across all areas:

```bash
./helper files reextractdiz --dry-run
```

#### Re-extract Behavior

- Only processes files with archive extensions recognized by `configs/archivers.json`
- Skips files not found on disk (reports `MISS`)
- Only updates records where the extracted DIZ differs from the current description
- Non-archive files are left unchanged

## Typical Workflows

### Importing a CD-ROM or Archive Collection

```bash
# 1. Preview what will be imported
./helper files import --dir /mnt/cdrom/files --area GENERAL --preserve-dates --dry-run

# 2. Run the actual import
./helper files import --dir /mnt/cdrom/files --area GENERAL --preserve-dates

# 3. Verify — check the BBS file listing or metadata directly
cat data/files/general/metadata.json | python3 -m json.tool | head -20
```

### Setting Up a New File Area with Files

```bash
# 1. Add area to configs/file_areas.json (or use existing area)

# 2. Import files
./helper files import --dir ~/bbs-files/utils --area UTILS --uploader "Sysop"

# 3. Restart the BBS to pick up new metadata
```

### Fixing Missing Descriptions

```bash
# Re-extract DIZ from all archives in all areas
./helper files reextractdiz

# Or target a specific area
./helper files reextractdiz --area GENERAL
```

## Supported Archive Formats

DIZ extraction supports all formats defined in `configs/archivers.json`:

| Format | Native | Notes |
|--------|--------|-------|
| ZIP    | Yes    | Built-in Go support, always available |
| 7Z     | No     | Requires `7z` binary |
| RAR    | No     | Requires `unrar` binary |
| ARJ    | No     | Requires `arj` binary |
| LHA/LZH | No   | Requires `lha` binary |

Native ZIP extraction reads FILE_ID.DIZ directly from the archive without extracting to disk. Non-native formats extract to a temporary directory, search for the DIZ file, then clean up.

## File Area Configuration Reference

File areas are defined in `configs/file_areas.json`. See [File Areas](file-areas.md) for full documentation on area configuration, metadata format, and template variables.

## Troubleshooting

### "file area not found"

The `--area` tag must match a tag in `configs/file_areas.json` (case-insensitive). Run without the flag to see available areas listed in the error output.

### DIZ not extracted from non-ZIP archives

Ensure the archiver is enabled in `configs/archivers.json` and the required external binary (e.g., `unrar`, `7z`) is installed and in PATH.

### Files imported but not visible in BBS

The BBS loads metadata at startup. Restart the BBS process after importing files, or the new files will appear on the next restart.

### Permission errors during import

Ensure the helper process has read access to the source directory and write access to the target area directory (`data/files/{area-path}/`).

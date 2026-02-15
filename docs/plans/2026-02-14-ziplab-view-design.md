# ZIPLAB_VIEW Design

## Goal

Interactive archive file viewer for the BBS file list. When a user presses V on an archive in LISTFILES, they enter an interactive viewer that shows numbered entries and allows single-file extraction via ZMODEM.

## Architecture

New `RunZipLabView` function in `internal/ziplab/viewer.go`. The V key in LISTFILES routes archives to this function instead of `viewFileByRecord`. All archive interaction lives in the `ziplab` package.

## Display Format

Built-in formatting with pipe codes. Paging for large archives.

```
--- Archive Contents: COOLUTIL.ZIP ---

  #   Size       Date       Name
 ---  ---------  ---------- --------------------------------
  1   12.4K      01/15/1995 README.TXT
  2   45.2K      01/15/1995 COOLUTIL.EXE
  3   1.2K       01/15/1995 FILE_ID.DIZ

 3 file(s), 58.8K total

[#]=Extract  [Q]=Quit
```

Template fallback: if `ZIPLIST.TOP`, `ZIPLIST.MID`, or `ZIPLIST.BOT` exist in the menu set's template directory, use those instead. Placeholders: `^NUM`, `^NAME`, `^SIZE`, `^DATE`.

## Interactive Loop

1. User enters file number → extract entry to temp file → ZMODEM send → clean up
2. User enters Q → return to file list
3. User presses ENTER → redisplay listing

## Extraction Flow

- Open ZIP, find entry by number
- Extract to temp dir (`os.MkdirTemp`)
- Call `transfer.ExecuteZmodemSend(session, tempFilePath)`
- Remove temp dir
- Return to prompt

## Routing Change (executor.go)

In the V key handler (~line 7189), check `IsSupportedArchive`:

```go
if e.FileMgr.IsSupportedArchive(fileToView.Filename) {
    filePath, _ := e.FileMgr.GetFilePath(fileToView.ID)
    ziplab.RunZipLabView(s, terminal, filePath, fileToView.Filename, outputMode)
} else {
    viewFileByRecord(e, s, terminal, &fileToView, outputMode)
}
```

## Function Signature

```go
func RunZipLabView(s ssh.Session, terminal *term.Terminal,
    filePath string, filename string, outputMode ansi.OutputMode)
```

No return values — sub-interaction that returns control to LISTFILES when done.

## Error Handling

- Corrupt/unreadable ZIP → display error, return to file list
- ZMODEM send failure → display error, continue in viewer loop
- Empty archive → display "empty archive" message, return

## Decisions

- Extraction: one file at a time (no batch/tagging)
- Templates: built-in with fallback to ZIPLIST.TOP/MID/BOT
- V key on archives routes to ZIPLAB_VIEW, non-archives stay with VIEW_FILE
- ZipLab IS the archive subsystem (preserving ViSiON/2 style)

# File Transfer System Plan

**Goal:** Implement a file transfer system tightly integrated with the BBS menu structure, supporting Zmodem for retro compatibility, user-specific file tagging for batch downloads, and archive file viewing/extraction (ZIPLab).

**Core Components:**

1.  **Data Structures (`internal/file/types.go`):** `[DONE]`
    *   `FileArea`: Defines logical file sections (Tag, Name, Description, Server Path, ACS for List/Upload/Download). Loaded from `configs/file_areas.json`.
    *   `FileRecord`: Defines metadata for each file (UUID, AreaID, Filename, Description, Size, UploadedAt, UploadedBy, DownloadCount). Stored possibly in `data/files/<AreaTag>/metadata.json` or a database.

2.  **File Management (`internal/file/manager.go`):** `[PARTIAL]`
    *   Loads/manages `FileArea` definitions.
    *   Loads/saves `FileRecord` metadata per area.
    *   Provides functions: `ListAreas`, `GetAreaByTag`, `GetAreaByID`, `GetFilesForArea`, `AddFileRecord`, `IncrementDownloadCount`, `GetFilePath`, `IsSupportedArchive(filename)`, `GetFileCountForArea`, `GetFilesForAreaPaginated`. (Core listing/counting/archive check functions implemented. ACS checks handled by callers).
    *   Handles filesystem operations (directory creation, temp file handling, temp extraction). (Basic directory creation used for setup).

3.  **User Tagging for Batch Download (`internal/user/user.go` & `UserMgr`):** `[DONE]`
    *   Add `TaggedFileIDs []uuid.UUID` to the `User` struct to store marked `FileRecord` IDs.
    *   Update `UserMgr` to save/load this slice.

4.  **Menu Integration (`menus/vX/...`):** `[PARTIAL]`
    *   `FILEM (.MNU, .CFG)`: Main entry point for file sections. Commands: Select Area (`A`/`*`), List Files (`F`), Batch Download (`B`), Quit (`Q`). (Exists, L command removed).
    *   `LISTFILEAR` Runnable: (Removed - Functionality merged into `SELECTFILEAREA`). List display logic refactored to internal helper `displayFileAreaList`.
    *   `SELECTFILEAREA` Runnable: `[DONE]` Displays list then prompts for Area Tag/ID, sets `currentUser.CurrentFileAreaID/Tag` after ACS check.
    *   `LISTFILES` Runnable: `[PARTIAL]`
        *   Displays paginated files in the current area using `FileManager.GetFilesForAreaPaginated`. Uses `FILELIST.TOP/MID/BOT` templates (showing details and tag marker `*`). (Implemented & working).
        *   Command Loop: (Basic N/P/Q navigation and Tagging implemented. Placeholders for D/U/V added).
            *   `D`ownload: Calls `DOWNLOADFILE`. (Placeholder added).
            *   `U`pload: Calls `UPLOADFILE`. (Placeholder added).
            *   `T`ag: Adds/Removes current file ID to/from `currentUser.TaggedFileIDs`. Displays `*` marker. (Implemented & working).
            *   `V`iew: If file is a supported archive, calls `ZIPLAB_VIEW`. Otherwise, shows full details. (Placeholder added).
            *   Sort / `J`ump / Page Navigation (`N`/`P`). (N/P Implemented).
            *   `A`rea Change / `Q`uit. (Quit Implemented, Area change now handled by 'A' option).
    *   `DOWNLOADFILE` Runnable: (Downloads *whole* files listed by `LISTFILES`)
        *   Takes file ID/number. Checks `ACSDownload`. Gets path via `FileManager`.
        *   Prompts protocol (Zmodem initially).
        *   Calls `executeZmodemSend(filePath)`.
        *   Increments download count. (Not started).
    *   `UPLOADFILE` Runnable:
        *   Checks `ACSUpload`. Prompts protocol (Zmodem).
        *   Calls `executeZmodemReceive(tempDir)`.
        *   On success, prompts for description, creates `FileRecord`, calls `FileManager.AddFileRecord`. (Not started).
    *   `BATCHDOWNLOAD` Runnable:
        *   Checks `currentUser.TaggedFileIDs`.
        *   Gets paths for tagged files via `FileManager`, checking `ACSDownload` for each.
        *   Prompts user to start Zmodem batch send.
        *   Calls `executeZmodemSend(filePath1, filePath2, ...)` (using `sz`'s multi-file capability).
        *   Clears `currentUser.TaggedFileIDs` and updates download counts on completion. (Not started).
    *   **`ZIPLAB_VIEW` Runnable (New):**
        *   Takes archive `FileRecord` ID as input. Gets path via `FileManager`.
        *   Uses Go archive library (e.g., `archive/zip`, `github.com/mholt/archiver`) to list contents.
        *   Displays internal file list using `ZIPLIST.TOP/MID/BOT` templates and pagination.
        *   Command Loop:
            *   `E`xtract/Download: Select internal file(s), extract to temp, call `executeZmodemSend` for temp file(s), cleanup temp.
            *   `Q`uit: Return to `LISTFILES`. (Not started).

5.  **Zmodem Implementation (`internal/transfer/zmodem.go` or similar):** `[TODO]`
    *   Requires `lrzsz` installed on the server.
    *   Helper functions `executeZmodemSend(...)` and `executeZmodemReceive(...)`.
    *   These functions wrap `exec.Command("sz", ...)` / `exec.Command("rz", ...)`.
    *   **Crucially:** Reuse/refactor PTY handling logic (raw mode, I/O piping) from the existing `DOOR:` handler in `executor.go`. (Not started).

6.  **Modern Options (Future):**
    *   Protocol selection prompt in `DOWNLOADFILE`/`UPLOADFILE` can be extended (e.g., "(H)TTP Link").
    *   HTTP download link generation would require a secondary HTTP service component.

**Phased Rollout Suggestion:**

1.  `[DONE]` Implement Structs (`FileArea`, `FileRecord`), User tagging field.
2.  `[PARTIAL]` Implement `FileManager` basics (loading areas, metadata stubs, `IsSupportedArchive`, file counts). (Core listing/counting/archive check methods done. ACS handled by caller).
3.  `[DONE]` Implement Menus & core Runnables (`FILEM`, `SELECTFILEAREA` including list display).
4.  `[DONE]` Implement User Tagging logic in `LISTFILES`.
5.  `[PARTIAL]` Add placeholders for Download/Upload/View in `LISTFILES`.
6.  `[TODO]` Implement Zmodem helpers (`executeZmodemSend/Receive`) reusing PTY logic.
7.  `[TODO]` Implement `UPLOADFILE` & `DOWNLOADFILE` runnables using Zmodem helpers.
8.  `[TODO]` Implement `BATCHDOWNLOAD` runnable.
9.  `[TODO]` Implement `ZIPLAB_VIEW` runnable (Archive Viewing/Extraction).
10. `[LATER]` Add features like sorting, new file scanning later. 
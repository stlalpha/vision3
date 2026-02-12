# Development Tasks

This file tracks active and planned development tasks for the ViSiON/3 BBS project.

## Current Development Tasks

### Core BBS Functionality

*   **[DONE] Implement Centralized Terminal Output Function:**
    *   **Goal:** Create a single function responsible for sending byte data (containing mixed ANSI codes and CP437 text) to the user's terminal, correctly converting CP437 to UTF-8 while preserving ANSI codes, based on detected/configured output mode.
    *   **Files:** `terminalio/writer.go`, `menu/executor.go`, `ansi/ansi.go`
    *   **Status:** DONE. `terminalio.WriteProcessedBytes` implemented and used throughout relevant parts of `menu/executor.go`.
*   **[DONE] Fix CP437/ANSI Rendering:** Resolve character encoding issues for files and prompts based on output mode.
*   **[DONE] Fix Last Callers Screen:**
    *   **Goal:** Ensure the Last Callers screen displays correct data and formatting.
    *   **Approach:** Implement persistent call history (`callhistory.json`), correctly parse templates (`LASTCALL.MID`), substitute placeholders (`^UN`, `^BA`, `|NU`), and process pipe codes.
    *   **Files:** `user/manager.go`, `menu/executor.go`, `main.go`.
    *   **Status:** DONE.
*   **[DONE] Refactor Data File Paths:**
    *   **Goal:** Move user data files (`users.json`, `callhistory.json`, `oneliners.json`) from `config/` to `data/`.
    *   **Files:** `main.go`, `user/manager.go`.
    *   **Status:** DONE.
*   **[DONE] Fix startup errors and path issues:** Resolved errors related to configuration file loading (`strings.json`, `theme.json`), log directory creation, SSH host key path, and initial oneliner loading.
*   **[DONE] Restore/Update Oneliner & Prompt Logic:** Re-implemented dynamic loading/saving for oneliners (`oneliners.json`), fixed yes/no lightbar prompt padding and dynamic positioning.
*   **[DONE] Implement VT100 Line Drawing for ANSI Art:**
    *   **Goal:** Reliably display CP437-based ANSI art (especially box characters) on modern terminals via SSH.
    *   **Approach:** Use VT100 line drawing mode escape sequences (`ESC(0`, `ESC(B`) and map CP437 characters to their VT100 equivalents.
    *   **Files:** `ansi/ansi.go` (implemented functions like `DisplayWithVT100LineDrawing`).
    *   **Status:** DONE. VT100 line drawing functions are implemented and available.
*   **[DONE] Refactor Menu System:**
    *   Implement the `executor` package to handle menu navigation based on `.MNU` and `.CFG` files.
    *   Connect menu actions to underlying functions (displaying ANSI, getting input, etc.).
    *   **Files:** `internal/menu/executor.go`, `menu/loader.go`, `menu/structs.go`.
    *   **Status:** DONE. Full menu system with lightbar support, ACS checking, and command execution.
*   **[DONE] User Authentication:**
    *   Implement user loading, password checking (bcrypt), and session management.
    *   Integrate with the login menu flow.
    *   **Files:** `user/manager.go`, `menu/executor.go`, `main.go`.
    *   **Status:** DONE. Full authentication system with bcrypt password hashing.

### Known Issues / Bugs

*   **[RESOLVED] Output Mode TODOs:** Several `// TODO: Pass correct mode` comments in DOOR error handlers currently default to `OutputModeCP437`. These need to be updated to accept the correct `outputMode` from context.
*   **[RESOLVED] Login Loop:** Fixed authentication flow to properly navigate to main menu after successful login.
*   **[RESOLVED] Local Package Resolution Errors:** Fixed import issues in `main.go`.

### Currently Active Features

*   **[DONE] Message System:**
    *   [x] Define data structures for message areas (`MessageArea` struct).
    *   [x] Implement persistence for message area definitions (`message_areas.json`).
    *   [x] Implement listing/navigation of message areas (`ListAreas`, `MSGMAIM` menu, `RUN:LISTMSGAR`).
    *   [x] Define data structures for messages (`Message` struct).
    *   [x] Implement persistence for message data (JSONL files per area).
    *   [x] Implement message posting command (`RUN:COMPOSEMSG`):
        *   [x] Create custom TUI editor (`internal/editor`) using `bubbletea`.
            *   Full-screen editor supporting multi-line input.
            *   Live parsing and rendering of `|XX` pipe codes within the editor UI.
            *   Preserves raw `|XX` codes in the final saved message text.
            *   Basic editor commands (Save, Abort).
        *   [x] Prompt for message subject after editor exits.
        *   [x] Save message using `MessageManager`.
    *   [x] Implement message reading command (`RUN:READMSGS`) with full pagination and navigation.
    *   [x] Implement newscan command (`RUN:NEWSCAN`) to check all areas for new messages.
*   **[DONE] File Areas:**
    *   [x] Define data structures for file areas (`FileArea` struct).
    *   [x] Implement persistence for file area definitions (`file_areas.json`).
    *   [x] Define data structures for files (`FileRecord` struct).
    *   [x] Implement persistence for file metadata (`metadata.json` per area).
    *   [x] Implement file listing (`RUN:LISTFILES`) with pagination and marking.
    *   [x] Implement file area listing (`RUN:LISTFILEAR`).
    *   [x] Implement file area selection (`RUN:SELECTFILEAREA`).
    *   [x] Implement file downloading with ZMODEM protocol (using external `sz` command).
    *   [ ] Implement file uploading (using external `rz` command).
    *   [ ] Implement file viewing (text files).
*   **[DONE] Door/External Program Support:**
    *   [x] Define and load external door configuration (`doors.json`).
    *   [x] Implement placeholder substitution (`{NODE}`, `{TIMELEFT}`, etc.).
    *   [x] Implement dropfile generation (`DOOR.SYS`, `CHAIN.TXT`).
    *   [x] Implement terminal raw mode switching for doors (using `RunCommandWithPTY`).
    *   [x] Implement actual external program execution with terminal attachment.

### Next Steps / TODOs

1.  **Address Remaining Output Mode TODOs:** Update the 2 remaining DOOR error handlers (lines 114, 129 in executor.go) to properly pass `outputMode`.
2.  **Implement File Upload:** Add support for file uploads using `rz` or similar.
3.  **Implement File Viewing:** Add ability to view text files and archives inline.
4.  **Add ACS Filtering:** Implement ACS filtering in message area listing (`runListMessageAreas`).
5.  **String Configuration:** Use configurable strings instead of hardcoded messages for the remaining TODOs in executor.go.
6.  **Complete Lightbar Support:** Detect lightbar menus via `.BAR` files or menu flags instead of hardcoding menu names.
7.  **Implement Additional Menu Commands:**
    *   [ ] `%%file.ans` support (File Inclusion) in prompts.
    *   [ ] `^P`, `^M`, `##` matching command types.
    *   [ ] Input fields (`type: "input"` in menu definitions).
    *   [ ] Forced Conference logic.
    *   [ ] AutoRun Once (`//`) and AutoRun Every (`~~`) command execution.

### Future Enhancements

*   **Chat/Paging System:**
    *   [ ] Implement inter-node chat
    *   [ ] Implement sysop paging
*   **Bulletins:**
    *   [ ] Define bulletin structure
    *   [ ] Implement bulletin viewing
*   **Voting Booths:**
    *   [ ] Design voting system
    *   [ ] Implement voting interface
*   **SysOp Utilities:**
    *   [ ] User editor
    *   [ ] System configuration editor
    *   [ ] File/Message area managers
*   **Additional Protocols:**
    *   [ ] Xmodem
    *   [ ] Ymodem
    *   [ ] Kermit

## Access Control System (ACS) Refinement

- [x] Implement robust ACS tokenizer to handle expressions without spaces (e.g., `S50&!L`).
- [x] Implement most ACS codes (S, F, U, V, L, A, D, E, H, P, T, W, Y, Z).
- [ ] Implement ACS codes requiring unavailable context (`B`, `C`, `I`, `X`) or add proper state tracking.
- [ ] Add unit tests for the ACS parser and evaluator.

## Tools / Utilities

- [ ] String Editor (`cmd/json-string-editor`): Create TUI tool to edit `config/strings.json`.
- [x] String Decoder (`cmd/strings-decoder`): Created tool to decode STRINGS.DAT.

## Documentation

- [x] Update project documentation (`README.md`, `documentation/`) to reflect current features and architecture.
- [ ] Add API documentation for extension developers.
- [ ] Create user manual for BBS operators.

## General

- [ ] Review and address remaining `// TODO:` comments throughout the codebase (approximately 28 remaining).
- [ ] Add comprehensive error handling and logging.
- [ ] Implement proper shutdown handlers.
- [ ] Add performance monitoring and optimization.

## Recently Completed (Archive)

*   [x] SSH Server implementation with connection handling
*   [x] User management with JSON persistence
*   [x] Configuration loading from JSON files
*   [x] ANSI parsing with full ViSiON/2 pipe code support
*   [x] Menu system with .MNU/.CFG binary format parsing
*   [x] Lightbar menu navigation
*   [x] Full ACS (Access Control System) implementation
*   [x] Last callers display with templates
*   [x] User statistics display
*   [x] Oneliner system
*   [x] User list display
*   [x] Message area foundations
*   [x] File area foundations
*   [x] Door/external program support
*   [x] ZMODEM file transfers
# Development Tasks

This file tracks active and planned development tasks for the ViSiON/3 BBS project.

## Recent Completions

*   **[DONE] Telnet TERM_TYPE Negotiation (2026-02-19):**
    *   **Goal:** Detect the connecting client's terminal type over telnet via RFC 1091 TERM_TYPE negotiation, so that SyncTerm, NetRunner, and other BBS clients are correctly identified and receive the right output mode (CP437 vs UTF-8) instead of always falling through to the "ansi" default.
    *   **Implementation:**
        *   Added two-phase `Negotiate()` in `TelnetConn`: phase 1 sends `DO TERM_TYPE` alongside existing options; phase 2 sends `SB TERM_TYPE SEND` if the client responded `WILL TERM_TYPE`, then drains the `IS <string>` response.
        *   Added state machine detection for `WILL TERM_TYPE` and subnegotiation handler for `OptTermType IS <string>`.
        *   Added `TermType()` getter on `TelnetConn` (defaults to `"ansi"` if client did not negotiate).
        *   `NewTelnetSessionAdapter` now uses `tc.TermType()` instead of hardcoded `"ansi"`, exposing the real terminal identifier via `Pty().Term` to the session handler's auto output-mode detection.
        *   Documented that the `xterm && cols > 80` NetRunner heuristic in `main.go` is SSH-specific; telnet clients now use TERM_TYPE directly.
    *   **Files:** `internal/telnetserver/telnet.go`, `internal/telnetserver/adapter.go`, `cmd/vision3/main.go`, `documentation/telnet-server.md`
    *   **Status:** COMPLETE.

*   **[DONE] Retrograde-Style One Liners (2026-02-13):**
    *   **Goal:** Replace legacy one-liner flow with Retrograde-style template UX while preserving anonymous posting and sysop traceability.
    *   **Implementation:**
        *   Reworked `RUN:ONELINER` to use `ONELINER.TOP/MID/BOT` template flow.
        *   Added JSON object model in `data/oneliners.json` with backward compatibility for legacy string entries.
        *   Added anonymous posting prompt and persisted poster identity fields (`posted_by_username`, `posted_by_handle`) for sysop visibility.
        *   Added configurable anonymous display label via `strings.json` key `anonymousName` (example: "Anonymous Coward").
        *   Fixed one-liner pipe code behavior and added support for `|CR` / `|DE` translation.
    *   **Files:** `internal/menu/executor.go`, `internal/ansi/ansi.go`, `internal/config/config.go`, `menus/v3/templates/ONELINER.*`, `configs/strings.json`, `templates/configs/strings.json`, `documentation/configuration.md`, `documentation/login-sequence.md`
    *   **Status:** COMPLETE. Feature is bd-tracked (`vision3-4ji`) and closed.

*   **[DONE] Configuration Hot Reload (2026-02-13):**
    *   **Goal:** Enable automatic reload of configuration files without requiring server restart
    *   **Implementation:**
        *   Created `ConfigWatcher` using fsnotify for file system monitoring
        *   Added thread-safe config setters/getters to `MenuExecutor` with `sync.RWMutex`
        *   Debounced file change events (500ms) to avoid multiple reloads
        *   Integrated into main.go startup sequence with graceful shutdown
    *   **Supported Configs:**
        *   `doors.json` - Door configurations hot reload automatically
        *   `login.json` - Login sequence changes apply immediately
        *   `strings.json` - String templates update without restart
        *   `theme.json` - Theme changes apply to new sessions
        *   `server.json` - Server settings (some require restart for ports/keys)
    *   **Features:**
        *   Atomic config updates with mutex protection
        *   Safe for active user sessions (doesn't affect users mid-session)
        *   Comprehensive logging of reload events
        *   Hot reload indicator in startup logs
    *   **Additional Work:**
        *   Fixed login sequence to support `DOOR:` command prefix
        *   Added XNEWS door to login.json sequence as proof-of-concept
        *   Configured XNEWS with STDIO mode (-X flag) and proper dropfile arguments
        *   Fixed double-keypress issue for standard I/O doors
    *   **Files:** `cmd/vision3/config_watcher.go`, `internal/menu/executor.go`, `internal/menu/door_handler.go`, `cmd/vision3/main.go`
    *   **Status:** COMPLETE. Fully integrated and tested with XNEWS door in login sequence.

*   **[DONE] Event Scheduler (2026-02-11):**
    *   **Goal:** Implement cron-style event scheduler for automated maintenance tasks (FTN mail polling, echomail tossing, backups, etc.)
    *   **Implementation:**
        *   Created `internal/scheduler` package with cron integration (robfig/cron v3)
        *   Event configuration via `configs/events.json` with cron syntax support
        *   Concurrency control with semaphore pattern (configurable max concurrent events)
        *   Per-event execution tracking to prevent overlapping runs
        *   Timeout support with context-based cancellation
        *   Event history persistence in `data/logs/event_history.json`
        *   Placeholder substitution ({TIMESTAMP}, {EVENT_ID}, {EVENT_NAME}, {BBS_ROOT}, {DATE}, {TIME}, {DATETIME})
        *   Comprehensive test suite with 100% pass rate
    *   **Features:**
        *   Standard cron syntax with seconds support
        *   Special schedules (@hourly, @daily, @weekly, @monthly, @yearly)
        *   Non-interactive batch execution (no PTY/TTY)
        *   Graceful shutdown with history persistence
        *   Detailed logging (INFO/WARN/ERROR/DEBUG levels)
    *   **Files:** `internal/scheduler/`, `internal/config/config.go`, `cmd/vision3/main.go`, `templates/configs/events.json`, `docs/event-scheduler.md`
    *   **Documentation:** Full user guide created at `docs/event-scheduler.md`
    *   **Status:** COMPLETE. Fully integrated, tested, and documented.

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

*   **[DONE] Message System (JAM Message Bases):**
    *   [x] Define data structures for message areas (`MessageArea` struct with JAM/FTN fields).
    *   [x] Implement persistence for message area definitions (`message_areas.json`).
    *   [x] Implement listing/navigation of message areas (`ListAreas`, `MSGMAIM` menu, `RUN:LISTMSGAR`).
    *   [x] Implement JAM binary message base (`internal/jam/`) with random-access read/write, CRC32 indexing, per-user lastread tracking.
    *   [x] Implement message posting command (`RUN:COMPOSEMSG`):
        *   [x] Create custom TUI editor (`internal/editor`) using `bubbletea`.
            *   Full-screen editor supporting multi-line input.
            *   Live parsing and rendering of `|XX` pipe codes within the editor UI.
            *   Preserves raw `|XX` codes in the final saved message text.
            *   Basic editor commands (Save, Abort).
        *   [x] Prompt for message subject after editor exits.
        *   [x] Save message using `MessageManager.AddMessage()` (JAM-backed).
    *   [x] Implement message reading command (`RUN:READMSGS`) with random-access navigation and JAM lastread.
    *   [x] Implement newscan command (`RUN:NEWSCAN`) using JAM per-user lastread tracking.
    *   [x] Implement newscan configuration (`RUN:NEWSCANCONFIG`) for per-user area tagging.
    *   [x] Implement message list view (`RUN:LISTMSGS`) with pagination, lightbar navigation, and status indicators.
    *   [x] Implement message area selection (`RUN:SELECTMSGAREA`).
    *   [x] Implement prompt-and-compose (`RUN:PROMPTANDCOMPOSEMESSAGE`).
*   **[DONE] Private Mail System:**
    *   [x] Create PRIVMAIL message area (ID 19, `area_type: "local"`).
    *   [x] Implement `AddPrivateMessage()` method with MSG_PRIVATE flag support.
    *   [x] Implement `RUN:SENDPRIVMAIL` - send private mail with recipient validation.
    *   [x] Implement `RUN:READPRIVMAIL` - read private mail with security filtering.
    *   [x] Implement `RUN:LISTPRIVMAIL` - list private mail messages.
    *   [x] Create EMAILM menu configuration (`.CFG` and `.MNU` files).
    *   [x] Integrate Email Menu into main menu (E key).
    *   [x] Security filter: users only see messages addressed to them (`IsPrivate() && To == currentUser`).
*   **[DONE] FTN Echomail System:**
    *   [x] Implement FTN Type-2+ packet library (`internal/ftn/`) - read/write .PKT files.
    *   [x] Implement built-in tosser (`internal/tosser/`) with inbound/outbound processing.
    *   [x] MSGID dupe checking with JSON-persisted dupe database.
    *   [x] SEEN-BY/PATH management with net compression.
    *   [x] Background polling at configurable interval.
    *   [x] Echomail-aware message composition (MSGID, tearline, origin line).
    *   [x] FTN configuration in `config.json` (address, links, paths).
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
*   **JAM Utilities (`cmd/jamutil`):**
    *   [ ] Base info/stats display
    *   [ ] Message packing/defragmentation
    *   [ ] Orphan cleanup and integrity checking
    *   [ ] Lastread reset/management
    *   [ ] Message purge by age/count
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
- [ ] JAM Utility (`cmd/jamutil`): Command-line tool for JAM base maintenance (pack, fix, purge, stats).

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

### Security & Connection Management (February 2025)

*   [x] **Connection Security System:**
    *   Connection tracker with max nodes and per-IP connection limits
    *   IP blocklist/allowlist support with CIDR range matching
    *   Template files for blocklist.txt and allowlist.txt in `templates/configs/`
    *   Integration with SSH and Telnet servers
    *   Comprehensive logging of blocked/allowed connections
*   [x] **IP-Based Authentication Lockout:**
    *   Refactored from user-based to IP-based lockout (prevents DoS attacks)
    *   Configurable failed login threshold and lockout duration
    *   In-memory tracking with automatic expiration
    *   Integration in authentication flow (both LOGIN and AUTHENTICATE)
    *   Added `IPLockoutChecker` interface for dependency injection
*   [x] **Auto-Reload for IP Filter Files:**
    *   File system watching using fsnotify library
    *   Automatic reload of blocklist.txt and allowlist.txt on file changes
    *   500ms debouncing to handle rapid successive edits
    *   Zero-downtime updates - no BBS restart required
    *   Thread-safe reload with mutex protection
    *   Comprehensive logging of reload events
*   [x] **Security Documentation:**
    *   Created comprehensive `docs/security.md` with examples and best practices
    *   Updated `docs/configuration.md` with IP filter configuration
    *   Updated `docs/installation.md` with security settings
    *   Documented auto-reload feature with usage examples

### Earlier Completed Features

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
*   [x] JAM binary message base implementation (`internal/jam/`)
*   [x] FTN packet library (`internal/ftn/`)
*   [x] Built-in FTN echomail tosser (`internal/tosser/`)
*   [x] Migration from JSONL to JAM message storage
*   [x] Per-user lastread tracking via JAM `.jlr` files (replaces UUID-based tracking)
*   [x] File area foundations
*   [x] Door/external program support
*   [x] ZMODEM file transfers
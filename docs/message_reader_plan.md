# Plan: Implementing the Message Reader (`runReadMsgs`)

> **Note:** This plan has been fully implemented. The message system now uses JAM binary message bases instead of JSONL files. See [message-areas.md](message-areas.md) for current documentation.

This document outlines the original steps used to implement the message reading functionality triggered by `RUN:READMSGS` from the `MSGMENU`.

**Goal:** Allow users to read messages sequentially within their currently selected message area, with basic navigation.

**Location:** The core logic resides in the `runReadMsgs` function within `internal/menu/executor.go`.

## Phase 1: Basic Message Reader Implementation [COMPLETED]

### Prerequisites: [COMPLETED]

1.  **User State:** The `currentUser *user.User` object must have `CurrentMessageAreaID` and `CurrentMessageAreaTag` populated correctly upon entering the `MSGMENU` (implemented).
2.  **Message Storage:** Messages are stored in JAM binary message bases under `data/msgbases/`.
3.  **Message Manager Method:** The `internal/message/manager.go:MessageManager` provides `GetMessage(areaID, msgNum int)` for random-access by message number, backed by JAM index files.
4.  **Display Templates:** Create necessary display templates within the active menu set's `templates` directory (e.g., `menus/v3/templates/`). Templates `MSGHEAD.TPL` and `MSGREAD.PROMPT` have been created.
    *   `MSGHEAD.TPL`: For displaying message header information (From, To, Subject, Date, Msg #, etc.).
    *   `MSGBODY.TPL`: (Optional) A simple container if needed, or just display the body directly.
    *   `MSGREAD.PROMPT`: The prompt displayed at the bottom while reading (e.g., showing commands like Next, Prev, Quit).

### Implementation Steps: [COMPLETED]

1.  **Function Definition (`runReadMsgs`):** [COMPLETED]
    *   Enhanced the existing placeholder function in `internal/menu/executor.go`.
    *   Added initial checks for user and area selection.

2.  **Data Fetching:** [COMPLETED]
    *   Gets `currentAreaID` from `currentUser.CurrentMessageAreaID`.
    *   Calls `MessageManager.GetMessagesForArea(currentAreaID)` to load messages.
    *   Handles errors and the case of no messages.

3.  **State Initialization:** [COMPLETED]
    *   Initializes `currentMessageIndex := 0`.
    *   Stores `totalMessages := len(messages)`.

4.  **Main Reader Loop:** [COMPLETED]
    *   Implemented a `for {}` loop.

5.  **Display Logic (Inside Loop):** [COMPLETED]
    *   Gets the current message.
    *   Loads `MSGHEAD.TPL` and `MSGREAD.PROMPT` templates.
    *   Creates and substitutes placeholders.
    *   Processes pipe codes in the message body.
    *   Clears screen and displays header, body (raw dump), and prompt using `terminalio.WriteProcessedBytes`.

6.  **Input Handling (Inside Loop):** [COMPLETED]
    *   Reads user input.
    *   Handles `N` (Next), `P` (Previous), and `Q` (Quit) commands.
    *   Includes basic boundary checks (first/last message).

7.  **Return:** [COMPLETED]
    *   Returns `nil, "", nil` after the loop breaks to continue in `MSGMENU`.

## Phase 2: Enhancements

*   **Word Wrapping/Pagination:** [COMPLETED] Breaks long lines and pauses display if message exceeds terminal height.
*   **New Scan Logic:** [COMPLETED] Per-user lastread tracking via JAM `.jlr` files. No more UUID-based tracking.
*   **Reply Functionality (`R`):** [COMPLETED] Quote original message, compose reply, link via MSGID/REPLY.
*   **Echomail Support:** [COMPLETED] Messages in echomail areas get MSGID, tearline, origin line, SEEN-BY/PATH automatically.
*   **FTN Tosser:** [COMPLETED] Built-in import/export of .PKT files for FidoNet echomail.
*   **Delete Functionality (`D`):** Not yet implemented.
*   **Change Area (`A`):** Not yet implemented.

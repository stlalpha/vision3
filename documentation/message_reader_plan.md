# Plan: Implementing the Message Reader (`runReadMsgs`)

This document outlines the steps required to implement the message reading functionality triggered by `RUN:READMSGS` from the `MSGMENU`.

**Goal:** Allow users to read messages sequentially within their currently selected message area, with basic navigation.

**Location:** The core logic resides in the `runReadMsgs` function within `internal/menu/executor.go`.

## Phase 1: Basic Message Reader Implementation [COMPLETED]

### Prerequisites: [COMPLETED]

1.  **User State:** The `currentUser *user.User` object must have `CurrentMessageAreaID` and `CurrentMessageAreaTag` populated correctly upon entering the `MSGMENU` (implemented).
2.  **Message Storage:** Messages for area `X` are expected to be stored in `data/messages_area_X.jsonl`.
3.  **Message Manager Method:** The `internal/message/manager.go:MessageManager` needs a method to retrieve messages for a specific area ID (e.g., `GetMessagesForArea(areaID int) ([]Message, error)`). This method has been implemented.
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

## Phase 2: Enhancements (Future Work)

*   **Word Wrapping/Pagination:** [COMPLETED] Implement logic to break long lines in the message body and pause display (`-- More --`) if the message exceeds terminal height.
*   **New Scan Logic:** [COMPLETED] Implemented new scan for current area ('R') and all areas ('N').
    *   [COMPLETED] Requires adding `LastReadMessageID` map to `user.User`.
    *   [COMPLETED] Modify data fetching to filter messages based on `LastReadMessageID`.
    *   [COMPLETED] Update the `LastReadMessageID` when the user quits the reader.
*   **Reply Functionality (`R`):**
    *   Call `editor.RunEditor`, potentially pre-filling the editor buffer with quoted text from `currentMsg`.
    *   Prompt for subject (perhaps defaulting to "Re: " + original subject).
    *   Construct and save the new reply message using `MessageManager.AddMessage`, linking it via `ReplyToID`.
*   **Jump Functionality (`J`):**
    *   Prompt user for a message number.
    *   Validate the number and update `currentMessageIndex`.
*   **Delete Functionality (`D`):**
    *   Check user permissions (e.g., SysOp level or message author).
    *   Implement message deletion/flagging in `MessageManager`.
*   **Change Area (`A`):** Prompt the user for a new Area ID/Tag, validate access, and restart the reader loop for the new area without returning to `MSGMENU`.
*   **Display Message Attributes:** Handle FidoNet-style attributes if present.
*   **More Robust Templating:** Use a more powerful templating engine if needed. 
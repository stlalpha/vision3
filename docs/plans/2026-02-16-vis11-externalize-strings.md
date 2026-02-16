# VIS-11: Externalize Hardcoded Strings Implementation Plan


**Goal:** Replace all hardcoded user-facing strings with configurable entries in strings.json, using original Vision 2 defaults.

**Architecture:** Every user-facing string lives in `StringsConfig` (loaded from `configs/strings.json`), accessed via `e.LoadedStrings.FieldName`. Hardcoded strings are replaced with loaded values plus fallback defaults. New V3-specific fields are added to the struct as needed.

**Tech Stack:** Go, JSON configuration

---

## Replacement Pattern

Every file follows the same pattern. For simple strings:

```go
// Before:
terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|09Hardcoded|07")), outputMode)

// After:
terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.SomeField)), outputMode)
```

For format strings with `%s`/`%d` args:

```go
// Before:
msg := fmt.Sprintf("|09Hello %s|07", name)

// After:
msg := fmt.Sprintf(e.LoadedStrings.HelloStr, name)
// where JSON default is "|09Hello %s|07"
```

The JSON default value is always the V2 original (or a sensible V3 default for new strings). If a field is empty string in JSON, the code should still work (empty output).

---

### Task 1: Populate Missing V2 String Defaults in strings.json

**Files:**
- Modify: `configs/strings.json`

The `StringsConfig` struct already has fields for all V2 strings (pages 5-9), but `strings.json` is missing values for ~80 of them. Add the V2 defaults from `STRINGS.PAS`.

**Step 1: Add missing V2 string values to strings.json**

Add these entries (after the existing `"wrongPassword"` entry at line 89, before `"yesPromptText"`). The values come directly from `FormatStrings` in `vision-2-bbs/SRC/STRINGS.PAS`:

```json
  "addBBSName": "|08B|07B|15S |13Name|05: ",
  "addBBSNumber": "|08B|07B|15S |13Number|05: ",
  "addBBSBaud": "|08H|07i|15ghest |13BPS|05: ",
  "addBBSSoftware": "|08B|07B|15S |13Software|05: ",
  "addExtendedBBSDescr": "|08U|07p|15load |08E|07x|15tended |08D|07e|15scription? @",
  "bbsEntryAdded": "|08·|02·|10· |15Your Entry has been added!",
  "viewNextDescrip": "|08V|07i|15ew |08N|07e|15xt? @",
  "joinedMsgConf": "|05(|13|NA|05) |08C|07o|15nference |08J|07o|15ined!",
  "joinedFileConf": "|05(|13|NA|05) |08C|07o|15nference |08J|07o|15ined!",
  "whosBeingVotedOn": "|08U|07s|15er |08N|07a|15me|09: |15|NA",
  "numYesVotes": "|08Y|07e|15s |08V|07o|15tes|09: |15|YV  |09(|13Required|01: |0510 Votes|09)",
  "numNoVotes": "|08N|07o |08V|07o|15tes|09 :|15 |NV  |09(|13Deletion|01:|05 10 Votes|09)",
  "nuvCommentHeader": "|08C|07o|15mments |08S|07o |08F|07a|15r|09...|CR",
  "enterNUVCommentPrompt": "|08E|07n|15ter |08a C|07o|15mment |08o|07n |15|NA (Cr/Aborts)|CR:",
  "nuvVotePrompt": "|09New User Voting |01- |09(|10?|02/|10Help|09): ",
  "yesVoteCast": "|04Y|12e|14s |09Vote Cast!",
  "noVoteCast": "|04N|12a|14h |09Vote Cast!",
  "noNewUsersPending": "|04N|12o |10NEW |04u|12s|14ers |04r|12i|14ght |04n|12o|14w!",
  "enterRumorTitle": "|05R|13u|14mor |05T|13i|14tle|08 : ",
  "addRumorAnonymous": "|05A|13n|14onymous? @",
  "enterRumorLevel": "|05M|13i|14nimum |05L|13e|14vel|08 :",
  "enterRumorPrompt": "|05E|13n|14ter |05R|13u|14mor |08(|15Cr|07/|15Abort|08)|CR:",
  "rumorAdded": "|04R|12u|14mor |04h|12a|14s |04b|12e|14en |04a|12d|14ded!",
  "listRumorsPrompt": "|09Valid Commands|01: |15R|07umors, |15S|07tats, |15B|07oth :",
  "sendMailToWho": "|08S|07e|15nd |08M|07a|15il |08T|07o? |09:",
  "carbonCopyMail": "|08C|07a|15rbon |08C|07o|15py |08T|07o? |08(|15Cr|07/|15None|08) |09: ",
  "notifyEMail": "|08R|07e|15turn |08R|07e|15ceipt? @",
  "eMailAnnouncement": "|09[|15C|09]reate/change · [|15D|09]elete · [|10Q|09]uit :",
  "sysOpNotHere": "|08C|07r|15immy |08i|15s |04B|12i|14z-E|08!|CR",
  "chatCostsHeader": "|09Chat Request costs |15|CC |09file points!",
  "stillWantToTry": "|09Still want to |08C|07h|15at? @",
  "notEnoughFPPoints": "|15Haha.. you are too poor!",
  "chatCallOff": "|08[|15C|08] |14Turns chat page OFF|08.",
  "chatCallOn": "|08[|15C|08] |14Turns chat page ON|08.",
  "feedbackSent": "|01F|09e|15edba|09c|01k |01S|09e|15n|09t|01!",
  "youHaveReadMail": "|08Y|07o|15u |08h|07a|15ve |08r|07e|15ad |14|TI |08b|07y |14|SB|08.",
  "deleteMailNow": "|08N|07u|15ke |08i|07t |08n|07o|15w? @",
  "currentMailNone": "|08C|07u|15rrent |08M|07a|15il: |10None",
  "currentMailWaiting": "|08C|07u|15rrent |08M|07a|15il: |09|TI |01by |09|AU",
  "pickMailHeader": "|08P|07i|15ck |08t|07h|15is |08h|07e|15ader? @",
  "listTitleType": "|09List Type |08(|15R|08)|07ange, |08(|15N|08)|07ext : ",
  "noMoreTitles": "|10No more titles!",
  "listTitlesToYou": "|09Only list messages directed at you? @",
  "subDoesNotExist": "Invalid Sub-Board!",
  "msgNewScanAborted": "|04N|12e|14wscan |04A|12b|14orted!",
  "msgReadingPrompt": "|09Reading |01(|13|BN|01) [|13|CB|05/|13|NB|01]|08: ",
  "currentSubNewScan": "|09Current |01(|13|BN|01)...",
  "jumpToMessageNum": "|09Jump to message # |01(|131|05/|13|NB|01) : ",
  "postingQWKMsg": "|09Posting Message on |01(|13|BN|01)",
  "totalQWKAdded": "|09Total Processed|08: |15|TO",
  "sendQWKPacketPrompt": "|08[|15Cr|08] |09Send QWK Packet, |08[|15Q|08]|09uits : ",
  "threadWhichWay": "|09Message Threading, |08[|15F|08]|09orward or |08[|15B|08]|09ackwards : ",
  "autoValidatingFile": "|05A|13u|15to-|05V|13a|15lidating |05(|15|FN|05)",
  "fileIsWorth": "|05F|13i|15le |05V|13a|15lue|07: |14|FP Point(s)",
  "grantingUserFP": "|05G|13r|15anting |05Y|13o|15u |14|FP |05f|13i|15le |05p|13o|15int(s)!",
  "fileIsOffline": "|09That file is |05(|13Offline|05)..",
  "crashedFile": "|09That file is not yet complete!",
  "badBaudRate": "|09Your baud rate is too |04L|12o|14w!",
  "unvalidatedFile": "|09That file is not yet |05(|13Validated|05)..",
  "specialFile": "|09You must have permission to download this file!",
  "noDownloadsHere": "|09No |05(|13Downloads|05) |09here!",
  "privateFile": "|09That file is not for |04Y|12o|14u!",
  "filePassword": "|09Enter File Password|CR|08:",
  "wrongFilePW": "|04W|12r|15ong |04P|12a|15ssword!",
  "fileNewScanPrompt": "|05N|13e|15w |05S|13c|15anning · |13|AN|15 · |05(|13?|05/|15Help|05): ",
  "invalidArea": "|04I|12n|14valid |04A|12r|14ea!",
  "untaggingBatchFile": "|05De-Tagging|13: |15|FN",
  "fileExtractionPrompt": "|09File Extract - |08[|15A|08]|09dd to Batch, |08[|15Q|08]|09uit :",
  "badUDRatio": "|09Bad UD Ratio (|15|RA%|09) - Needed (|15|RR%|09)",
  "badUDKRatio": "|09Bad UD K Ratio (|15|RA%|09) - Required (|15|RR%|09)",
  "exceededDailyKBLimit": "|09Adding this exceeds your DL 'K' Limit of (|15|KL|09)",
  "filePointCommision": "|09Giving |13|NA|09 a commision of |14|FP|09 point(s)!",
  "successfulDownload": "|05S|13u|07c|15cess! |05(|13|FN|05) CPS: |15|CP |05Cost: |15|CO",
  "fileCrashSave": "|09File Crash! Save Partial File? @",
  "invalidFilename": "|04I|12n|14valid |04F|12i|14lename!",
  "alreadyEnteredFilename": "|09You already entered that!",
  "fileAlreadyExists": "|09File exists in current directory!",
  "enterFileDescription": "|05D|13e|07s|15cription|07: ",
  "extendedUploadSetup": "|05S|13e|15tup · |05(|13A|05)dd PW, (|13P|05)rivate, or (|13Cr|05) : ",
  "reEnterFileDescrip": "|05D|13e|07s|15cription of |05(|13|FN|05): ",
  "notifyIfDownloaded": "|05N|13o|15tify |05y|13o|15u |05w|13h|15en |FN |05i|13s |05l|13e|15eched? @",
  "fiftyFilesMaximum": "|04You can only tag up to 50 files!",
  "youCantDownloadHere": "|04You can't download here!",
  "fileAlreadyMarked": "|09File is already marked!",
  "notEnoughFP": "|04N|12o|15t |04e|12n|15ough |04f|12i|15le |04p|12o|15ints!",
  "fileAreaPassword": "|04A|12r|15ea |04P|12a|15ssword|08: "
```

**Step 2: Verify JSON parses correctly**

Run: `go build ./...`
Expected: clean build (JSON is only parsed at runtime, but the build validates Go code)

**Step 3: Commit**

```bash
git add configs/strings.json
git commit -m "feat(VIS-11): populate missing V2 string defaults in strings.json"
```

---

### Task 2: Externalize Chat and Page Strings

**Files:**
- Modify: `internal/config/config.go` (add new fields to StringsConfig)
- Modify: `configs/strings.json` (add V3-specific chat/page entries)
- Modify: `internal/menu/chat.go` (replace hardcoded strings)
- Modify: `internal/menu/page.go` (replace hardcoded strings)

**Step 1: Add new StringsConfig fields**

Add these fields to `StringsConfig` in `internal/config/config.go` (after the `QuotePrefix` field, before `DefColor1`):

```go
	// Chat strings (V3-specific)
	ChatHeader         string `json:"chatHeader"`
	ChatSeparator      string `json:"chatSeparator"`
	ChatUserEntered    string `json:"chatUserEntered"`
	ChatUserLeft       string `json:"chatUserLeft"`
	ChatSystemPrefix   string `json:"chatSystemPrefix"`
	ChatMessageFormat  string `json:"chatMessageFormat"`

	// Page strings (V3-specific)
	PageOnlineNodesHeader string `json:"pageOnlineNodesHeader"`
	PageNodeListEntry     string `json:"pageNodeListEntry"`
	PageWhichNodePrompt   string `json:"pageWhichNodePrompt"`
	PageMessagePrompt     string `json:"pageMessagePrompt"`
	PageMessageFormat     string `json:"pageMessageFormat"`
	PageSent              string `json:"pageSent"`
	PageCancelled         string `json:"pageCancelled"`
	PageInvalidNode       string `json:"pageInvalidNode"`
	PageSelfError         string `json:"pageSelfError"`
	PageNodeOffline       string `json:"pageNodeOffline"`
```

**Step 2: Add JSON entries to configs/strings.json**

```json
  "chatHeader": "|12Teleconference Chat|07  |08(type |15/Q|08 to quit)|07",
  "chatSeparator": "|08────────────────────────────────────────────────────────────────────────────────|07",
  "chatUserEntered": "|10%s has entered chat|07",
  "chatUserLeft": "|09%s has left chat|07",
  "chatSystemPrefix": " |08*** %s|07",
  "chatMessageFormat": "|11<%s|11>|07 %s",
  "pageOnlineNodesHeader": "\r\n|12Online Nodes:|07\r\n",
  "pageNodeListEntry": " |15Node %d|07: %s\r\n",
  "pageWhichNodePrompt": "\r\n|07Page which node? (|15Q|07 to cancel): ",
  "pageMessagePrompt": "|07Message: ",
  "pageMessageFormat": "|09Page from |15%s|09: %s|07",
  "pageSent": "|10Page sent to Node %d.|07\r\n",
  "pageCancelled": "|09Page cancelled.|07\r\n",
  "pageInvalidNode": "|09Invalid node number.|07\r\n",
  "pageSelfError": "|09You can't page yourself.|07\r\n",
  "pageNodeOffline": "|09That node is not online.|07\r\n"
```

**Step 3: Replace hardcoded strings in chat.go**

In `internal/menu/chat.go`, replace each hardcoded string with its `e.LoadedStrings` equivalent:

- Line 43: `header := ...` → `header := e.LoadedStrings.ChatHeader`
- Line 48: `sep := ...` → `sep := e.LoadedStrings.ChatSeparator`
- Line 85-86: `fmt.Sprintf("|10%s has entered chat|07", handle)` → `fmt.Sprintf(e.LoadedStrings.ChatUserEntered, handle)`
- Line 89: `inputSep := ...` → `inputSep := e.LoadedStrings.ChatSeparator`
- Line 112: `fmt.Sprintf("|09%s has left chat|07", handle)` → `fmt.Sprintf(e.LoadedStrings.ChatUserLeft, handle)`
- Line 147: same as line 112
- `formatChatMessage`: line 160: `fmt.Sprintf(" |08*** %s|07", msg.Text)` → `fmt.Sprintf(e.LoadedStrings.ChatSystemPrefix, msg.Text)`
- `formatChatMessage`: line 162: `fmt.Sprintf("|11<%s|11>|07 %s", ...)` → `fmt.Sprintf(e.LoadedStrings.ChatMessageFormat, ...)`

Note: `formatChatMessage` needs access to `e.LoadedStrings`. Change its signature to accept the format strings, or make it a method on MenuExecutor. Simplest: pass the format strings from the caller.

**Step 4: Replace hardcoded strings in page.go**

In `internal/menu/page.go`, replace each hardcoded string:

- Line 28: `"|12Online Nodes:|07"` → `e.LoadedStrings.PageOnlineNodesHeader` (remove `\r\n` wrapper, it's in the JSON value)
- Line 41: `" |15Node %d|07: %s\r\n"` → `e.LoadedStrings.PageNodeListEntry`
- Line 46: `"|07Page which node?..."` → `e.LoadedStrings.PageWhichNodePrompt`
- Line 61: `"|09Invalid node number.|07"` → `e.LoadedStrings.PageInvalidNode`
- Line 67: `"|09You can't page yourself.|07"` → `e.LoadedStrings.PageSelfError`
- Line 74: `"|09That node is not online.|07"` → `e.LoadedStrings.PageNodeOffline`
- Line 80: `"|07Message: "` → `e.LoadedStrings.PageMessagePrompt`
- Line 90: `"|09Page cancelled.|07"` → `e.LoadedStrings.PageCancelled`
- Line 96: `"|09Page from |15%s|09: %s|07"` → `e.LoadedStrings.PageMessageFormat`
- Line 100: `"|10Page sent to Node %d.|07"` → `e.LoadedStrings.PageSent`

**Step 5: Build and test**

Run: `go build ./... && go test ./internal/menu/ ./internal/config/ ./internal/chat/`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/config/config.go configs/strings.json internal/menu/chat.go internal/menu/page.go
git commit -m "feat(VIS-11): externalize chat and page strings to strings.json"
```

---

### Task 3: Externalize Newuser Strings

**Files:**
- Modify: `internal/config/config.go` (add new fields)
- Modify: `configs/strings.json` (add entries)
- Modify: `internal/menu/newuser.go` (replace hardcoded strings)

**Step 1: Add new StringsConfig fields**

```go
	// Newuser strings (V3-specific)
	NewUserLocationPrompt    string `json:"newUserLocationPrompt"`
	NewUserPasswordTooShort  string `json:"newUserPasswordTooShort"`
	NewUserPasswordMismatch  string `json:"newUserPasswordMismatch"`
	NewUserInvalidRealName   string `json:"newUserInvalidRealName"`
	NewUserTooManyAttempts   string `json:"newUserTooManyAttempts"`
	NewUserAccountCreated    string `json:"newUserAccountCreated"`
	NewUserCreationError     string `json:"newUserCreationError"`
```

**Step 2: Add JSON entries**

```json
  "newUserLocationPrompt": "|08G|07r|15oup|08/|07L|15ocation |09: ",
  "newUserPasswordTooShort": "\r\n|09Password must be at least 3 characters.|07\r\n",
  "newUserPasswordMismatch": "\r\n|09They don't match!|07\r\n",
  "newUserInvalidRealName": "\r\n|05Please enter your |10first |05and |10last |05name.|07\r\n",
  "newUserTooManyAttempts": "\r\n|05Too many invalid attempts.|07\r\n",
  "newUserAccountCreated": "\r\n|15Your account has been created but requires |13SysOp validation|15.\r\n|08Please call back later to check your access.|07\r\n",
  "newUserCreationError": "\r\n|13Error creating account. Please try again later.|07\r\n"
```

**Step 3: Replace hardcoded strings in newuser.go**

- Line 143: `errMsg := "\r\n|13Error..."` → `errMsg := e.LoadedStrings.NewUserCreationError`
- Line 159: `validationMsg := "\r\n|15Your account..."` → `validationMsg := e.LoadedStrings.NewUserAccountCreated`
- Line 265: `errMsg := "\r\n|05Too many..."` → `errMsg := e.LoadedStrings.NewUserTooManyAttempts`
- Line 304: `msg := "\r\n|09Password must be..."` → `msg := e.LoadedStrings.NewUserPasswordTooShort`
- Line 324: `msg := "\r\n|09They don't match..."` → `msg := e.LoadedStrings.NewUserPasswordMismatch`
- Line 334: `errMsg := "\r\n|09Too many..."` → `errMsg := e.LoadedStrings.NewUserTooManyAttempts`
- Line 371: `msg := "\r\n|05Please enter..."` → `msg := e.LoadedStrings.NewUserInvalidRealName`
- Line 380: `errMsg := "\r\n|05Too many..."` → `errMsg := e.LoadedStrings.NewUserTooManyAttempts`
- Line 393: `prompt := "|08G|07r|15oup..."` → `prompt := e.LoadedStrings.NewUserLocationPrompt`

**Step 4: Build and test**

Run: `go build ./... && go test ./internal/menu/ ./internal/config/`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go configs/strings.json internal/menu/newuser.go
git commit -m "feat(VIS-11): externalize newuser strings to strings.json"
```

---

### Task 4: Externalize System Stats Strings

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/strings.json`
- Modify: `internal/menu/system_stats.go`

**Step 1: Add new StringsConfig fields**

```go
	// System stats strings
	StatsBBSName      string `json:"statsBBSName"`
	StatsSysOp        string `json:"statsSysOp"`
	StatsVersion      string `json:"statsVersion"`
	StatsTotalUsers   string `json:"statsTotalUsers"`
	StatsTotalCalls   string `json:"statsTotalCalls"`
	StatsTotalMsgs    string `json:"statsTotalMessages"`
	StatsTotalFiles   string `json:"statsTotalFiles"`
	StatsActiveNodes  string `json:"statsActiveNodes"`
	StatsDate         string `json:"statsDate"`
	StatsTime         string `json:"statsTime"`
```

**Step 2: Add JSON entries**

```json
  "statsBBSName": " |07BBS Name:       |15%s",
  "statsSysOp": " |07SysOp:          |15%s",
  "statsVersion": " |07Version:        |15ViSiON/3 v%s",
  "statsTotalUsers": " |07Total Users:    |15%s",
  "statsTotalCalls": " |07Total Calls:    |15%s",
  "statsTotalMessages": " |07Total Messages: |15%s",
  "statsTotalFiles": " |07Total Files:    |15%s",
  "statsActiveNodes": " |07Active Nodes:   |15%s |07/ |15%s",
  "statsDate": " |07Date:           |15%s",
  "statsTime": " |07Time:           |15%s"
```

**Step 3: Replace hardcoded strings in system_stats.go**

Replace lines 76-88 (the `lines` slice) to use `e.LoadedStrings`:

```go
	lines := []string{
		fmt.Sprintf(e.LoadedStrings.StatsBBSName, tokens["BBSNAME"]),
		fmt.Sprintf(e.LoadedStrings.StatsSysOp, tokens["SYSOP"]),
		fmt.Sprintf(e.LoadedStrings.StatsVersion, tokens["VERSION"]),
		"",
		fmt.Sprintf(e.LoadedStrings.StatsTotalUsers, tokens["TOTALUSERS"]),
		fmt.Sprintf(e.LoadedStrings.StatsTotalCalls, tokens["TOTALCALLS"]),
		fmt.Sprintf(e.LoadedStrings.StatsTotalMsgs, tokens["TOTALMSGS"]),
		fmt.Sprintf(e.LoadedStrings.StatsTotalFiles, tokens["TOTALFILES"]),
		fmt.Sprintf(e.LoadedStrings.StatsActiveNodes, tokens["ACTIVENODES"], tokens["MAXNODES"]),
		"",
		fmt.Sprintf(e.LoadedStrings.StatsDate, tokens["DATE"]),
		fmt.Sprintf(e.LoadedStrings.StatsTime, tokens["TIME"]),
	}
```

**Step 4: Build and test**

Run: `go build ./... && go test ./internal/menu/ ./internal/config/`

**Step 5: Commit**

```bash
git add internal/config/config.go configs/strings.json internal/menu/system_stats.go
git commit -m "feat(VIS-11): externalize system stats strings to strings.json"
```

---

### Task 5: Externalize User Config Strings

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/strings.json`
- Modify: `internal/menu/user_config.go`

Read `internal/menu/user_config.go` thoroughly first. Identify all hardcoded pipe-coded strings and format strings. Add fields for each to StringsConfig, add JSON entries, replace in code. Follow the same pattern as previous tasks.

Key strings to externalize:
- Screen width/height prompts and confirmation messages
- Terminal type set message
- Text input prompts with current values
- Field updated messages
- Color selection prompts and confirmations
- Display format strings for settings panel (screen width, height, terminal type, etc.)
- Color input prompt
- Error saving messages

**Step 1-5: Same pattern as above**

**Commit:**
```bash
git add internal/config/config.go configs/strings.json internal/menu/user_config.go
git commit -m "feat(VIS-11): externalize user config strings to strings.json"
```

---

### Task 6: Externalize Message System Strings

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/strings.json`
- Modify: `internal/menu/message_reader.go`
- Modify: `internal/menu/message_scan.go`
- Modify: `internal/menu/message_list.go`

Read all three files thoroughly first. For message_list.go, the box-drawing strings use CP437 characters and ANSI sequences - these should still be externalized but with careful attention to the byte values.

Key strings to externalize per file:

**message_reader.go:**
- End of messages, mail reply not implemented, message list not implemented
- Scroll percentage format, board status line format
- Quote prefix/continuation, origin line format
- No thread found, jump to message prompt

**message_scan.go:**
- Filter display lines (date, to, from, range, update newscan, which areas)
- Range start/end prompts
- Scanning progress format
- Newscan saved message

**message_list.go:**
- Box drawing components (top, title, separator, empty, page info, help, bottom)
- Message line selected/normal formats
- Page footer formats

**Step 1-5: Same pattern as above. Build and test after each file.**

**Commit:**
```bash
git add internal/config/config.go configs/strings.json internal/menu/message_reader.go internal/menu/message_scan.go internal/menu/message_list.go
git commit -m "feat(VIS-11): externalize message system strings to strings.json"
```

---

### Task 7: Externalize File Viewer, Door Handler, Matrix, and Conference Strings

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/strings.json`
- Modify: `internal/menu/file_viewer.go`
- Modify: `internal/menu/door_handler.go`
- Modify: `internal/menu/matrix.go`
- Modify: `internal/menu/conference_menu.go`

Read all four files. Key strings:

**file_viewer.go:**
- Filename prompt, file not found, viewing header

**door_handler.go:**
- Error messages (system file creation, door not found, generic)
- Door info display lines (name, type, command, directory, dropfile, DOS commands)

**matrix.go:**
- Account validated message, account not validated message

**conference_menu.go:**
- Conference not found error, current area display, no accessible conferences/areas messages

**Step 1-5: Same pattern**

**Commit:**
```bash
git add internal/config/config.go configs/strings.json internal/menu/file_viewer.go internal/menu/door_handler.go internal/menu/matrix.go internal/menu/conference_menu.go
git commit -m "feat(VIS-11): externalize file viewer, door, matrix, and conference strings"
```

---

### Task 8: Externalize Executor Strings

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/strings.json`
- Modify: `internal/menu/executor.go`

This is the largest file (9157 lines). Read it carefully and identify ALL hardcoded pipe-coded strings. There are ~60+ strings covering:

- Login flow (goodbye, password accepted/incorrect, too many attempts, cancelled, IP throttle)
- Error messages (screen file, menu loading, command not found, door errors)
- Mail notification, pending validation count
- Admin user details (handle, username, real name, phone, group, flags, validated, level, etc.)
- Admin actions (validate, unvalidate, ban, delete confirmations and results)
- Message area operations (invalid area, access denied, no messages, total count, read prompt)
- File operations (upload prompt/progress, download results, area navigation)
- File transfer messages (ZMODEM initiation, completion, errors)
- Area navigation messages (area set, no accessible areas, divider line)
- User list messages (no users found, no pending validation, all validated)
- Message posting (error saving, posted successfully, no areas available)
- Access denied, page navigation messages

Add fields, JSON entries, and replace hardcoded strings following the same pattern.

**Important:** For admin strings with complex formatting (multi-column user details), keep the format strings as-is in JSON. They contain `%s`, `%d`, `%t`, `%-Ns` format verbs that must be preserved exactly.

**Step 1-5: Same pattern**

**Commit:**
```bash
git add internal/config/config.go configs/strings.json internal/menu/executor.go
git commit -m "feat(VIS-11): externalize executor strings to strings.json"
```

---

### Task 9: Update Templates and Final Verification

**Files:**
- Modify: `templates/configs/strings.json`

**Step 1: Copy strings.json to templates**

```bash
cp configs/strings.json templates/configs/strings.json
```

**Step 2: Full build and test**

Run: `go build ./... && go test ./...`
Expected: All tests PASS, clean build

**Step 3: Verify string count**

```bash
# Count JSON keys in strings.json
jq 'keys | length' configs/strings.json
```

Expected: ~373 entries (101 original + 80 V2 populated + ~192 V3 new). Admin UI strings (user editor, validation browser) are deferred to future work.

**Step 4: Commit**

```bash
git add templates/configs/strings.json
git commit -m "feat(VIS-11): sync templates/strings.json with updated configs"
```

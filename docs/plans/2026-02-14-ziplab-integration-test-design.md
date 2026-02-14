# ZipLab Integration Test Design

## Context

VIS-49: The ZipLab pipeline has 44 unit tests covering individual steps and the orchestrator with synthetic data. Missing: an end-to-end integration test that uses the real project template files and verifies the final ZIP binary state after pipeline execution.

## Design

**File:** `internal/ziplab/integration_test.go`

**Structure:** Parent `TestZipLabPipeline_Integration` with `t.Run()` subtests. Skips if real assets not found (`../../menus/v3/ansi/ZIPLAB.*`, `../../ziplab/*`).

**Shared setup helper** creates a `Processor` configured with the real template files from `ziplab/` and parses the real ZIPLAB.NFO.

### Subtests

**1. `happy_path`** — ZIP containing `FILE_ID.DIZ`, `VENDOR.TXT` (ad file from REMOVE.TXT), `readme.txt`

Assertions:
- `PipelineResult.Success == true`
- `PipelineResult.Description` matches DIZ content
- All step results are `StatusPass`
- Final ZIP binary contains: comment from ZCOMMENT.TXT, BBS.AD entry, original readme.txt, FILE_ID.DIZ

Additionally runs `DisplayPipeline` with `bytes.Buffer`, real NFO, and real ZIPLAB.ANS — verifies output contains ANSI content and cursor-positioning escapes.

**2. `corrupt_zip`** — Invalid bytes as `.zip`

Assertions:
- `PipelineResult.Success == false`
- Only 1 step result (integrity), status `StatusFail`
- No further steps executed

**3. `no_diz`** — Valid ZIP with `readme.txt` only, no FILE_ID.DIZ

Assertions:
- `PipelineResult.Success == true`
- `PipelineResult.Description == ""`
- Final ZIP still has comment and BBS.AD added

## Real Assets Used

- `menus/v3/ansi/ZIPLAB.ANS` — ANSI art background
- `menus/v3/ansi/ZIPLAB.NFO` — Step coordinate config
- `ziplab/REMOVE.TXT` — Ad filename patterns
- `ziplab/ZCOMMENT.TXT` — ZIP comment template
- `ziplab/BBS.AD` — BBS advertisement file

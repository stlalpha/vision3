# Testing Roadmap

## Goals
- Cover critical session flows (login, menu navigation, disconnect) with deterministic unit and integration tests.
- Validate config tooling (`cmd/vision3-config`, `internal/configtool/*`, `pkg/goturbotui`) through UI-layer tests and component isolation.
- Provide automated regression checks for encoding/output modes and ANSI asset loading.

## Unit Testing Priorities
- `internal/menu`: table-driven tests for ACS evaluation, menu loading, and command execution fallbacks.
- `internal/session`: mock SSH sessions to verify session state transitions, call-record persistence, and node bookkeeping.
- `pkg/goturbotui`: component-level tests (focus handling, redraw requests, modal stack operations) using fake screens.
- `internal/config`: config parsing/validation with malformed input fixtures in `internal/config/testdata/`.
- `internal/message` & `internal/file`: CRUD operations against in-memory or temp-dir data providers.

## Integration & End-to-End
- Spin up the BBS via `go test ./test/integration` harness that launches `cmd/vision3` with ephemeral configs and asserts login/menu flows via PTY session mocks (consider `expect`-style sequences).
- CLI smoke tests for `cmd/vision3-config` using `tcell` simulation to drive menu selection and confirm JSON persistence.
- Build script validation: run `scripts/build/build-dist.sh --dry-run` inside CI sandbox with mocks for GPG to ensure packaging succeeds without artifacts committed back.

## Tooling & Infrastructure
- Introduce `Makefile` or `mage` targets: `make test`, `make integration`, `make lint` for simplified CI wiring.
- Adopt `golangci-lint` configuration aligned with Go 1.24.2; ensure `go vet ./...` runs in CI.
- Capture coverage via `go test ./... -coverprofile=coverage.out`; gate merges on minimum threshold (target 60% initial, adjust upward).

## Data & Fixtures
- Store fixtures under `testdata/` per package; seed default menu/config snapshots for deterministic expectations.
- Use temporary directories during tests; never touch `data/` runtime files.
- Provide helper builders for fake users/messages to avoid repeating JSON scaffolding.

## CI Recommendations
- Set up GitHub Actions (or preferred runner) with jobs: `lint`, `unit`, `integration`, `build-dist`.
- Cache Go modules to keep pipelines fast; upload coverage reports as artifacts.
- Require green status before PR merge and surface failed test logs for quick triage.

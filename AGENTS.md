# Repository Guidelines

## Project Structure & Module Organization
- Application entry point is `cmd/vision3`; building here emits the SSH BBS binary.
- Core logic lives under `internal/` (`internal/menu`, `internal/message`, `internal/terminalio`, etc.); keep new domains in focused subpackages.
- Runtime config JSON sits in `configs/`; mutable data under `data/` stays uncommitted.
- ANSI art, menus, and templates live in `menus/v3/`; keep `.cfg`, `.mnu`, and `.ans` files in sync.
- Reference docs and task notes are stored in `docs/` and `tasks/`.

## Build, Test, and Development Commands
- `./setup.sh` — generate keys, seed data directories, and build `vision3`; rerun after schema tweaks.
- `go build ./cmd/vision3` — compile the server binary against the current workspace.
- `go run ./cmd/vision3` — launch the SSH service locally (defaults to port 2222).
- `go test ./...` — run Go unit tests; add `-cover` for coverage signals before review.
- `go vet ./...` — lint for common Go issues; run after significant refactors.

## Coding Style & Naming Conventions
- Target Go 1.24.2 standards: tabs for indentation, `gofmt` on every change, and package comments on exported APIs.
- Exported identifiers use UpperCamelCase; internal helpers stay lowerCamelCase; prefer descriptive filenames like `ansi_parser.go`.
- Keep `internal/<domain>` boundaries tight and avoid leaking UI concerns into backend packages.
- Mirror naming across menu assets so `.cfg`, `.mnu`, and `.ans` variants stay traceable.

## Testing Guidelines
- Place tests beside code (`internal/menu/menu_test.go`) and follow `TestFeatureBehavior` naming.
- Use table-driven cases for ACS checks, ANSI parsing, and transfer state machines.
- Store reusable fixtures under `testdata/` and avoid mutating files in `data/` during tests.
- Ensure `go test ./...` passes cleanly before opening a PR.

## Commit & Pull Request Guidelines
- Write short, imperative commit subjects (`Add login banner`, `Fix session ACS guard`) with optional bodies for deep context.
- Scope PRs to a single feature or bugfix; describe user-visible impact, config updates, and tests executed.
- Link related issues or task IDs and attach ANSI/menu previews when assets change.
- Call out follow-up steps (e.g., rerunning `./setup.sh`, rotating keys) in PR notes.

## Configuration & Security Notes
- Never commit generated keys or runtime artifacts from `data/`; use the samples under `configs/` instead.
- Review `configs/config.json` defaults before deploying and document new fields in the same directory.
- When testing locally, reserve TCP port 2222 and keep test credentials temporary.

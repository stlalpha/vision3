# ViSiON/3 Development Guidelines

## Core Philosophy
- **Simplicity**: Prioritize simple, clear, and maintainable solutions. Avoid unnecessary complexity or over-engineering.
- **Iterate**: Prefer iterating on existing, working code rather than building entirely new solutions from scratch, unless fundamentally necessary or explicitly requested.
- **Focus**: Concentrate efforts on the specific task assigned. Avoid unrelated changes or scope creep.
- **Quality**: Strive for a clean, organized, well-tested, and secure codebase.

## Project Context & Understanding

### Documentation First
Always check for and review relevant project documentation before starting any task:
- `README.md` — Project overview, setup, patterns, technology stack
- `docs/architecture.md` — System architecture, component relationships
- `docs/technical.md` — Technical specifications, established patterns
- `tasks/tasks.md` — Current development tasks, requirements

If documentation is missing, unclear, or conflicts with the request, ask for clarification.

### Architecture Adherence
- Understand and respect module boundaries, data flow, system interfaces, and component dependencies outlined in `docs/architecture.md`.
- Validate that changes comply with the established architecture. Warn and propose compliant solutions if a violation is detected.

### Pattern & Tech Stack Awareness
- Reference `README.md` and `docs/technical.md` to understand and utilize existing patterns and technologies.
- Exhaust options using existing implementations before proposing new patterns or libraries.

## Task Execution & Workflow

### Systematic Change Protocol
Before making significant changes:
1. **Identify Impact**: Determine affected components, dependencies, and potential side effects.
2. **Plan**: Outline the steps. Tackle one logical change or file at a time.
3. **Verify Testing**: Confirm how the change will be tested. Add tests if necessary before implementing.

### Progress Tracking
- Keep `docs/status.md` updated with task progress (in-progress, completed, blocked), issues encountered, and completed items.
- Update `tasks/tasks.md` upon task completion or if requirements change during implementation.

## Code Quality & Style

### Golang Guidelines
- Follow the official Go style guide. Use `gofmt` for formatting.
- Document code with proper Go comments.
- Follow RESTful or gRPC design principles as per project standards.
- Write clean, well-organized code with meaningful variable and function names.

### Small Files & Components
- Keep files under 300 lines. Refactor proactively.
- Break down large packages into smaller, single-responsibility modules.
- Follow Go's package organization principles.

### General Rules
- **DRY**: Actively look for and reuse existing functionality. Refactor to eliminate duplication.
- **No Custom Build Systems**: Use Go's built-in tools and established project tooling.
- **Linting**: Use `gofmt`, `golint`, and `go vet` for consistent code quality. Follow `golangci-lint` rules if configured.
- **Pattern Consistency**: Adhere to established project patterns. Don't introduce new ones without discussion. If replacing an old pattern, ensure the old implementation is fully removed.
- **File Naming**: Use `snake_case` for Go file names (e.g., `user_profile.go`). Follow Go conventions for package names (lower case, short, concise). Avoid "temp", "refactored", "improved" in permanent file names.
- **No One-Time Scripts**: Do not commit one-time utility scripts into the main codebase.

## Refactoring
- Refactor to improve clarity, reduce duplication, simplify complexity, or adhere to architectural goals.
- When refactoring, look for duplicate code, similar components/files, and opportunities for consolidation.
- Modify existing files directly. Do not duplicate files and rename them (e.g., `profile_service_v2.go`).
- After refactoring, ensure all callers, dependencies, and integration points function correctly. Run relevant tests.

## Testing & Validation

### Test-Driven Development (TDD)
- **New Features**: Outline tests, write failing tests, implement code, refactor.
- **Bug Fixes**: Write a test reproducing the bug before fixing it.

### Comprehensive Tests
- Use Go's standard `testing` package for unit tests and benchmarks.
- Use `testify` or other approved testing libraries if established in the project.
- Implement integration tests for API endpoints and service layers.
- All tests must pass before committing or considering a task complete.
- Use mock data only within test environments.

## Debugging & Troubleshooting
- **Fix the Root Cause**: Prioritize fixing the underlying issue, rather than masking or handling it.
- Check application logs for errors, warnings, or relevant information.
- Use the project's established logging framework with structured logging.
- Check the `fixes/` directory for documented solutions to similar past issues before deep-diving.
- Document complex fixes in `fixes/` with a descriptive `.md` file detailing the problem, investigation steps, and solution.

## Security
- Keep sensitive logic, validation, and data manipulation on the backend.
- Always validate and sanitize incoming data. Implement proper error handling to prevent information leakage.
- Be mindful of security implications when adding or updating dependencies.
- Never hardcode secrets or credentials. Use environment variables or secure secrets management.

## Version Control
- Commit frequently with clear, atomic messages.
- Keep the working directory clean; ensure no unrelated or temporary files are staged.
- Use `.gitignore` effectively.
- Follow the project's established branching strategy.
- Never commit `.env` files. Use `.env.example` for templates.

## Documentation Maintenance
- If code changes impact architecture, technical decisions, established patterns, or task status, update the relevant documentation (`README.md`, `docs/architecture.md`, `docs/technical.md`, `tasks/tasks.md`, `docs/status.md`).

## Go-Specific Best Practices

### Error Handling
- Always check error returns and handle them appropriately.
- Use meaningful error messages and consider custom error types for complex scenarios.
- Follow project conventions for error wrapping (`fmt.Errorf`, `errors.Wrap`, etc.).

### Concurrency
- Use goroutines and channels with caution and care.
- Implement proper synchronization mechanisms (mutex, waitgroup, etc.).
- Be aware of race conditions and deadlocks.

### Resource Management
- Always close resources (files, connections, etc.) using `defer` when appropriate.
- Implement proper context handling for cancellation and timeouts.

### Interface Design
- Keep interfaces small and focused.
- Define interfaces at the consuming package, not the implementing package (except for well-established patterns).

### Project Structure
- Follow the project's established structure for new packages and components.
- Use Go modules for dependency management.
- Organize code by functionality rather than type.

# Git Hooks for Vision3

This directory contains Git hooks that help maintain code quality.

## Available Hooks

### pre-commit

Runs before each commit to:
- ✅ Verify Go code builds successfully (prevents commits with compilation errors)
- ⚠️ Check code formatting with `gofmt` (warning only)

## Installation

### Option 1: Automatic (Recommended)

Run this command from the project root:

```bash
git config core.hooksPath .githooks
```

This configures Git to use the hooks in this directory instead of `.git/hooks/`.

### Option 2: Manual Copy

Copy the hooks to your local `.git/hooks/` directory:

```bash
cp .githooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## Bypassing Hooks

If you need to bypass the pre-commit hook (not recommended):

```bash
git commit --no-verify -m "your message"
```

## What Gets Checked

### Build Check (Blocking)
- Runs `go build ./...` to verify all packages compile
- Commit is **blocked** if build fails
- Helps catch:
  - Missing imports (like the `fmt` import issue)
  - Syntax errors
  - Type mismatches
  - Undefined variables/functions

### Format Check (Warning)
- Runs `gofmt -l .` to check formatting
- Commit **continues** even if formatting issues exist
- Displays list of unformatted files
- Run `gofmt -w .` to auto-format all files

## Benefits

1. **Catch errors early** - Find compilation issues before pushing
2. **Save CI time** - Don't wait for CI to tell you about build failures
3. **Better commits** - Ensure code quality before it enters history
4. **Team consistency** - Everyone uses the same checks

## Troubleshooting

### Hook not running?
- Check hook is executable: `ls -la .git/hooks/pre-commit` or `ls -la .githooks/pre-commit`
- Make executable: `chmod +x .git/hooks/pre-commit`
- Verify Git config: `git config core.hooksPath`

### Build fails during commit?
- Fix the compilation errors shown in the output
- Run `go build ./...` manually to see full error details
- Use `git commit --no-verify` as a last resort (not recommended)

## Maintenance

To update hooks for all team members:
1. Edit the hook in `.githooks/`
2. Commit the changes
3. Team members pull the changes
4. Hooks are automatically updated if using `core.hooksPath`

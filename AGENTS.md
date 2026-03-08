# Agent Instructions

This project uses **bd** (beads) for issue tracking and **Claude Code team mode** (`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`) for multi-agent coordination.

---

## Multi-Agent Team Roles

### Team Lead
- Spawns specialist subagents for parallel work
- Creates and assigns beads issues (`bd create`, `bd update <id> --assignee=<name>`)
- Monitors overall progress via `bd list`, `bd stats`, `bd blocked`
- Owns the session close/push protocol (see "Landing the Plane" below)
- Resolves blockers and dependency conflicts between agents

### Subagent (Specialist)
- Claims assigned tasks: `bd update <id> --status=in_progress`
- Works on a focused scope — avoids touching files outside the assigned task
- Reports completion via SendMessage to the team lead
- Closes finished issues: `bd close <id>`
- Does **not** push to remote — that's the team lead's responsibility

---

## Task Coordination Protocol

### Claiming Work
```bash
bd ready                                   # Find available (unblocked) tasks
bd show <id>                               # Review full issue details
bd update <id> --status=in_progress        # Claim it (set yourself as in-progress)
bd update <id> --assignee=<agent-name>     # Or assign to a specific agent
```

**Rule**: Only one agent should claim any given task. Check `bd list --status=in_progress` before claiming.

### Completing Work
```bash
bd close <id>                              # Mark complete
bd close <id1> <id2> ...                   # Close multiple at once
```

Always message the team lead when you close a task so they can unblock dependents.

### Creating Issues
```bash
bd create --title="..." --description="..." --type=task|bug|feature --priority=2
bd dep add <issue> <depends-on>            # Wire up dependencies
```

**Priority**: 0=critical, 1=high, 2=medium, 3=low, 4=backlog.
**Never use**: `bd edit` (opens $EDITOR, blocks agent).

---

## Communication Protocol

Use `SendMessage` to coordinate:

| Situation | Action |
|---|---|
| Task complete | Message team lead with task ID and summary |
| Blocked by another agent's work | Message team lead with blocker |
| Found a dependency not in beads | Message team lead before creating new issues |
| Need clarification on scope | Message team lead before writing code |

**Keep messages short**: task ID, status, any blockers. Don't repeat context the lead already has.

---

## Git Safety in Team Mode

Multiple agents working in parallel risk commit conflicts. Follow these rules:

1. **Scope your files**: Coordinate with the team lead on which files/packages each agent owns
2. **Commit atomically**: Commit only files related to your assigned task
3. **Pull before committing**:
   ```bash
   git pull --rebase
   git add <specific-files>
   git commit -m "feat(scope): description"
   ```
4. **Never force-push** — coordinate with team lead if push fails
5. **Subagents do not push** — stage and commit only; team lead pulls and pushes at session end

---

## Beads Quick Reference

```bash
bd ready                    # Available work (no blockers)
bd list --status=open       # All open issues
bd list --status=in_progress  # Active work
bd show <id>                # Full issue details + dependencies
bd update <id> --status=in_progress
bd close <id>
bd search <keyword>
bd stats                    # Project health overview
bd blocked                  # Blocked issues
bd dolt push                # Sync beads to remote
```

---

## Landing the Plane (Session Completion)

**Team lead responsibility** — work is NOT complete until `git push` succeeds.

1. **File issues for remaining work** — create beads issues for anything unfinished
2. **Run quality gates** — `go test ./...`, `gofmt -l .`, `go vet ./...`
3. **Close finished issues** — `bd close <id1> <id2> ...`
4. **Sync and push**:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status   # MUST show "up to date with origin"
   ```
5. **Verify** — all changes committed and pushed
6. **Hand off** — summarize state for the next session via `bd update` notes or `bd remember "..."`

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing — that leaves work stranded locally
- NEVER say "ready to push when you are" — YOU must push
- If push fails, resolve and retry until it succeeds

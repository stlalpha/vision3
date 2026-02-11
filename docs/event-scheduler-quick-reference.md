# Event Scheduler Quick Reference

Quick reference for ViSiON/3 BBS Event Scheduler configuration.

For complete documentation, see [Event Scheduler Guide](event-scheduler.md).

## Quick Start

1. **Copy template:**
   ```bash
   cp templates/configs/events.json configs/
   ```

2. **Enable scheduler:**
   Edit `configs/events.json`, set `"enabled": true`

3. **Configure events:**
   Add your events to the `events` array

4. **Start BBS:**
   Scheduler starts automatically

## Minimal Event

```json
{
  "id": "my_event",
  "name": "My Event",
  "schedule": "*/15 * * * *",
  "command": "/path/to/command",
  "timeout_seconds": 300,
  "enabled": true
}
```

## Multiple Arguments

Each argument is a separate array element:

```json
{
  "command": "/usr/local/bin/binkd",
  "args": [
    "-P",
    "21:4/158@fsxnet",
    "-D",
    "data/ftn/binkd.conf"
  ]
}
```

**NOT like this:** `"args": ["-P 21:4/158@fsxnet"]` ❌

## Cron Schedule Cheat Sheet

| Schedule | Description |
|----------|-------------|
| `* * * * *` | Every minute |
| `*/5 * * * *` | Every 5 minutes |
| `*/15 * * * *` | Every 15 minutes |
| `*/30 * * * *` | Every 30 minutes |
| `0 * * * *` | Every hour (at minute 0) |
| `0 */2 * * *` | Every 2 hours |
| `0 0 * * *` | Daily at midnight |
| `0 3 * * *` | Daily at 3 AM |
| `0 0 * * 0` | Weekly (Sunday midnight) |
| `0 0 1 * *` | Monthly (1st at midnight) |
| `@hourly` | Every hour |
| `@daily` | Once per day at midnight |
| `@weekly` | Weekly on Sunday |
| `@monthly` | Monthly on the 1st |

## Placeholders

| Placeholder | Example | Description |
|-------------|---------|-------------|
| `{TIMESTAMP}` | `1707667200` | Unix timestamp |
| `{EVENT_ID}` | `ftn_poll` | Event identifier |
| `{EVENT_NAME}` | `FTN Poll` | Event name |
| `{BBS_ROOT}` | `/home/bbs/vision3` | BBS directory |
| `{DATE}` | `2026-02-11` | Current date |
| `{TIME}` | `14:30:00` | Current time |
| `{DATETIME}` | `2026-02-11 14:30:00` | Date and time |

## Common Patterns

### FTN Mail Poll

```json
{
  "id": "ftn_poll",
  "name": "Poll FTN Mail",
  "schedule": "*/15 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-p"],
  "timeout_seconds": 300,
  "enabled": true
}
```

### HPT Toss

```json
{
  "id": "hpt_toss",
  "name": "Toss Echomail",
  "schedule": "@hourly",
  "command": "/usr/local/bin/hpt",
  "args": ["toss"],
  "timeout_seconds": 600,
  "enabled": true
}
```

### Daily Backup

```json
{
  "id": "backup",
  "name": "Daily Backup",
  "schedule": "0 2 * * *",
  "command": "/usr/bin/tar",
  "args": [
    "-czf",
    "/backups/bbs-{DATE}.tar.gz",
    "{BBS_ROOT}/data"
  ],
  "timeout_seconds": 7200,
  "enabled": true
}
```

### Shell Script

```json
{
  "id": "maintenance",
  "name": "Maintenance Script",
  "schedule": "0 4 * * *",
  "command": "/bin/sh",
  "args": [
    "-c",
    "cd {BBS_ROOT} && ./cleanup.sh && ./optimize.sh"
  ],
  "timeout_seconds": 1800,
  "enabled": true
}
```

## Environment Variables

```json
{
  "command": "/usr/local/bin/myapp",
  "environment_vars": {
    "CONFIG_FILE": "/etc/myapp.conf",
    "LOG_LEVEL": "debug",
    "BBS_PATH": "{BBS_ROOT}"
  }
}
```

## Working Directory

```json
{
  "command": "./script.sh",
  "working_directory": "/opt/scripts",
  "args": ["--verbose"]
}
```

## Timeout

```json
{
  "timeout_seconds": 300  // 5 minutes
}
```

Set to `0` for no timeout (not recommended).

## Troubleshooting

### Event Not Running

Check logs for:
```
INFO: Event scheduler started with N events
INFO: Event 'event_id' scheduled: @hourly
```

If missing:
- Verify `"enabled": true` (root level AND event level)
- Check cron schedule syntax
- Review BBS startup logs for errors

### Event Failing

Check logs for:
```
ERROR: Event 'event_id' failed with exit code 1
ERROR: Event 'event_id' stderr: [error message]
```

Fix:
- Use absolute path for command
- Test command manually
- Check file permissions
- Verify working directory exists

### Event Timeout

```
ERROR: Event 'event_id' timed out after 300s
```

Fix:
- Increase `timeout_seconds`
- Optimize the command
- Check for hung processes

## Log Locations

- **BBS logs:** `data/logs/vision3.log`
- **Event history:** `data/logs/event_history.json`

## Log Levels

```
INFO: Event started/completed
WARN: Event skipped (already running, concurrency limit)
ERROR: Event failed/timed out
DEBUG: Event output (stdout/stderr)
```

## Concurrency

```json
{
  "max_concurrent_events": 3  // Global limit
}
```

- Per-event: Won't run if already executing
- Global: Max N events running simultaneously
- Exceeded: Event skipped with warning

## Disable Scheduler

Set in `configs/events.json`:

```json
{
  "enabled": false
}
```

BBS starts normally, scheduler doesn't run.

# Event Scheduler

The ViSiON/3 BBS includes a built-in event scheduler that can automatically execute maintenance tasks, poll FTN mail, and run other scheduled operations using cron-style syntax.

## Overview

The event scheduler provides:

- **Cron-style scheduling**: Standard 5-field cron syntax
- **Concurrency control**: Limit how many events can run simultaneously
- **Timeout protection**: Prevent runaway processes with configurable timeouts
- **Event history**: Track execution statistics, success/failure rates
- **Placeholder substitution**: Dynamic values in commands and arguments
- **Non-interactive execution**: All events run in batch mode (no TTY/PTY)

## Configuration

Event scheduling is configured in `configs/events.json`.

### Basic Structure

```json
{
  "enabled": true,
  "max_concurrent_events": 3,
  "events": [
    {
      "id": "my_event",
      "name": "My Scheduled Event",
      "schedule": "*/15 * * * *",
      "command": "/path/to/command",
      "args": ["arg1", "arg2"],
      "working_directory": "/path/to/workdir",
      "timeout_seconds": 300,
      "enabled": true,
      "environment_vars": {
        "VAR_NAME": "value"
      }
    }
  ]
}
```

### Configuration Fields

#### Root Configuration

- **enabled** (boolean): Enable/disable the entire scheduler
- **max_concurrent_events** (integer): Maximum number of events that can run simultaneously (default: 3)
- **events** (array): List of event configurations

#### Event Configuration

- **id** (string, required): Unique identifier for the event
- **name** (string, required): Human-readable name for logging
- **schedule** (string, required): Cron schedule expression (see below)
- **command** (string, required): Path to the executable
- **args** (array): Command-line arguments
- **working_directory** (string): Directory to run the command in
- **timeout_seconds** (integer): Maximum execution time (0 = no timeout)
- **enabled** (boolean): Enable/disable this specific event
- **environment_vars** (object): Environment variables to set
- **run_after** (string): Event ID that must complete before this event runs (future feature)
- **delay_after_seconds** (integer): Delay after run_after event completes (future feature)

## Cron Schedule Syntax

The scheduler uses standard 5-field cron syntax:

```text
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

### Examples

- `* * * * *` - Every minute
- `*/15 * * * *` - Every 15 minutes
- `0 * * * *` - Every hour (at minute 0)
- `0 3 * * *` - Daily at 3:00 AM
- `0 0 * * 0` - Weekly on Sunday at midnight
- `0 0 1 * *` - Monthly on the 1st at midnight

### Special Schedules

The scheduler also supports special keywords:

- `@hourly` - Run once an hour (same as `0 * * * *`)
- `@daily` - Run once a day at midnight (same as `0 0 * * *`)
- `@weekly` - Run once a week on Sunday at midnight
- `@monthly` - Run once a month on the 1st at midnight
- `@yearly` - Run once a year on January 1st at midnight

## Placeholders

Event commands and arguments can include placeholders that are substituted at runtime:

- `{TIMESTAMP}` - Unix timestamp (seconds since epoch)
- `{EVENT_ID}` - Event identifier
- `{EVENT_NAME}` - Event name
- `{BBS_ROOT}` - BBS installation directory
- `{DATE}` - Current date (YYYY-MM-DD)
- `{TIME}` - Current time (HH:MM:SS)
- `{DATETIME}` - Current date and time (YYYY-MM-DD HH:MM:SS)

### Example with Placeholders

```json
{
  "id": "backup",
  "name": "Daily Backup",
  "schedule": "@daily",
  "command": "/usr/local/bin/backup.sh",
  "args": [
    "{BBS_ROOT}/data",
    "/backups/bbs-{DATE}.tar.gz"
  ],
  "timeout_seconds": 3600,
  "enabled": true
}
```

## Quick Reference

### Cron Schedule Patterns

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

### Finding Command Paths

```bash
# Check if binkd is installed
which binkd

# Common locations
/usr/bin/binkd
/usr/local/bin/binkd
~/bin/binkd
```

### Testing Commands Manually

Before adding to scheduler, test commands manually:

```bash
cd /home/bbs/git/vision3
/usr/local/bin/binkd -p -D data/ftn/binkd.conf
./v3mail toss --config configs --data data
./v3mail scan --config configs --data data
./v3mail ftn-pack --config configs --data data
```

## Common Use Cases

### FTN Mail Polling (Binkd)

**Simple poll all nodes every 30 minutes:**

```json
{
  "id": "binkd_poll",
  "name": "Poll FTN Mail",
  "schedule": "*/30 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-p", "-D", "{BBS_ROOT}/data/ftn/binkd.conf"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 300,
  "enabled": true
}
```

**Poll specific node every 15 minutes:**

```json
{
  "id": "binkd_poll_fsxnet",
  "name": "Poll FSXNet Hub",
  "schedule": "*/15 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-P", "21:4/158@fsxnet", "-D", "{BBS_ROOT}/data/ftn/binkd.conf"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 300,
  "enabled": true
}
```

### Echomail Tossing (v3mail)

**Toss inbound echomail every hour:**

```json
{
  "id": "v3mail_toss",
  "name": "Toss Echomail",
  "schedule": "@hourly",
  "command": "/usr/bin/go",
  "args": ["run", "{BBS_ROOT}/cmd/v3mail", "toss", "--config", "{BBS_ROOT}/configs", "--data", "{BBS_ROOT}/data"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 600,
  "enabled": true
}
```

**Scan outbound messages every 5 minutes:**

```json
{
  "id": "v3mail_scan",
  "name": "Scan Outbound Echomail",
  "schedule": "*/5 * * * *",
  "command": "/usr/bin/go",
  "args": ["run", "{BBS_ROOT}/cmd/v3mail", "scan", "--config", "{BBS_ROOT}/configs", "--data", "{BBS_ROOT}/data"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 300,
  "enabled": true
}
```

**Pack outbound bundles (runs 2 minutes after scan):**

```json
{
  "id": "v3mail_ftn_pack",
  "name": "Pack Outbound Bundles",
  "schedule": "2,7,12,17,22,27,32,37,42,47,52,57 * * * *",
  "command": "/usr/bin/go",
  "args": ["run", "{BBS_ROOT}/cmd/v3mail", "ftn-pack", "--config", "{BBS_ROOT}/configs", "--data", "{BBS_ROOT}/data"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 300,
  "enabled": true
}
```

### Full FTN Mail Workflow

Process mail in stages with timing offsets:

```json
{
  "events": [
    {
      "id": "binkd_poll",
      "name": "Poll FTN Mail",
      "schedule": "0,30 * * * *",
      "command": "/usr/local/bin/binkd",
      "args": ["-p", "-D", "{BBS_ROOT}/data/ftn/binkd.conf"],
      "timeout_seconds": 300,
      "enabled": true
    },
    {
      "id": "v3mail_toss",
      "name": "Toss Mail After Poll",
      "schedule": "5,35 * * * *",
      "command": "/usr/bin/go",
      "args": ["run", "{BBS_ROOT}/cmd/v3mail", "toss", "--config", "{BBS_ROOT}/configs", "--data", "{BBS_ROOT}/data"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 600,
      "enabled": true
    },
    {
      "id": "v3mail_scan",
      "name": "Scan Outbound After Toss",
      "schedule": "7,37 * * * *",
      "command": "/usr/bin/go",
      "args": ["run", "{BBS_ROOT}/cmd/v3mail", "scan", "--config", "{BBS_ROOT}/configs", "--data", "{BBS_ROOT}/data"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 300,
      "enabled": true
    },
    {
      "id": "v3mail_ftn_pack",
      "name": "Pack Outbound Bundles",
      "schedule": "10,40 * * * *",
      "command": "/usr/bin/go",
      "args": ["run", "{BBS_ROOT}/cmd/v3mail", "ftn-pack", "--config", "{BBS_ROOT}/configs", "--data", "{BBS_ROOT}/data"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 300,
      "enabled": true
    }
  ]
}
```

This creates a workflow:
- **:00, :30** - Poll mail from uplinks
- **:05, :35** - Toss received mail into message bases
- **:07, :37** - Scan JAM bases for new outbound messages
- **:10, :40** - Pack outbound `.pkt` files into ZIP bundles for binkd pickup

### Nightly Message Base Maintenance

ViSiON/3 uses `v3mail` for JAM message base maintenance. The recommended nightly sequence runs in three stages:

| Time | Stage | Purpose |
|------|-------|---------|
| 2:00 AM | `fix --repair --all` | Check integrity and repair malformed headers |
| 2:15 AM | `purge --all` | Remove messages exceeding per-area age/count limits |
| 2:30 AM | `pack --all` | Defragment bases and reclaim space from deleted messages |

Purge limits (`max_age`, `max_messages`) are configured per area in `message_areas.json`. See [Message Purge Configuration](../messages/message-areas.md#message-purge-configuration) for details.

```json
{
  "events": [
    {
      "id": "nightly_msgbase_fix",
      "name": "Nightly Message Base Integrity Check",
      "schedule": "0 2 * * *",
      "command": "/usr/bin/go",
      "args": ["run", "{BBS_ROOT}/cmd/v3mail", "fix", "--repair", "--all"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 3600,
      "enabled": true
    },
    {
      "id": "nightly_msgbase_purge",
      "name": "Nightly Message Base Purge",
      "schedule": "15 2 * * *",
      "comment": "Purge limits are read from max_msg_age/max_msgs in message_areas.json",
      "command": "/usr/bin/go",
      "args": ["run", "{BBS_ROOT}/cmd/v3mail", "purge", "--all"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 3600,
      "enabled": true
    },
    {
      "id": "nightly_msgbase_pack",
      "name": "Nightly Message Base Pack",
      "schedule": "30 2 * * *",
      "command": "/usr/bin/go",
      "args": ["run", "{BBS_ROOT}/cmd/v3mail", "pack", "--all"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 3600,
      "enabled": true
    }
  ]
}
```

### Nightly User Purge

Permanently remove soft-deleted user accounts that have exceeded the retention period configured in `configs/config.json` (`deletedUserRetentionDays`, default 30 days). Safe to run daily — accounts not yet past the retention window are left untouched.

```json
{
  "id": "purge_deleted_users",
  "name": "Nightly Deleted User Purge",
  "schedule": "0 3 * * *",
  "command": "{BBS_ROOT}/helper",
  "args": ["users", "purge"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 60,
  "enabled": true
}
```

See [Purging Deleted Users](../users/user-management.md#purging-deleted-users) for full details including CLI usage and the `--dry-run` preview flag.

### Nightly Maintenance

Run maintenance script at 3 AM:

```json
{
  "id": "maintenance",
  "name": "Nightly Maintenance",
  "schedule": "0 3 * * *",
  "command": "/bin/sh",
  "args": ["-c", "cd {BBS_ROOT} && ./cleanup.sh && ./optimize.sh"],
  "timeout_seconds": 1800,
  "enabled": true
}
```

### Daily Backup

Daily backup at 2 AM with date in filename:

```json
{
  "id": "backup",
  "name": "Daily Backup",
  "schedule": "0 2 * * *",
  "command": "/usr/bin/tar",
  "args": ["-czf", "/backups/bbs-{DATE}.tar.gz", "{BBS_ROOT}/data"],
  "timeout_seconds": 7200,
  "enabled": true
}
```

## Event History

The scheduler tracks execution history in `data/logs/event_history.json`:

```json
[
  {
    "event_id": "ftn_poll",
    "last_run": "2026-02-11T14:30:00Z",
    "last_status": "success",
    "last_duration_ms": 1234,
    "run_count": 50,
    "success_count": 48,
    "failure_count": 2
  }
]
```

History is updated after each event execution and saved on scheduler shutdown.

## Logging

Event execution is logged to the BBS log with the following patterns:

- `INFO: Event scheduler started with N events`
- `INFO: Event 'event_id' (Event Name) scheduled: @hourly`
- `INFO: Event 'event_id' (Event Name) started`
- `INFO: Event 'event_id' (Event Name) completed in 1.234s (exit code: 0)`
- `ERROR: Event 'event_id' (Event Name) failed with exit code 1`
- `ERROR: Event 'event_id' (Event Name) timed out after 300s`
- `WARN: Event 'event_id' (Event Name) skipped: already running`
- `WARN: Event 'event_id' (Event Name) skipped: max concurrent events reached (3)`

Debug output (stdout/stderr) is logged at DEBUG level when events complete.

## Concurrency Control

The scheduler includes two levels of concurrency control:

1. **Global limit**: `max_concurrent_events` limits total concurrent executions
2. **Per-event limit**: An event will not start if it's already running

### Behavior

- If an event is scheduled to run but is already executing, it is skipped
- If the global concurrency limit is reached, new events are skipped
- Skipped executions are logged as warnings

### Example

With `max_concurrent_events: 2`:

```text
Time  Event A  Event B  Event C  Result
----  -------  -------  -------  ------
0:00  Start    -        -        A runs (1/2 slots)
0:01  Running  Start    -        B runs (2/2 slots)
0:02  Running  Running  Start    C skipped (limit reached)
0:03  Done     Running  Start    C runs (A freed slot)
```

## Error Handling

### Non-Fatal Errors (logged, scheduler continues)

- Event fails to start (command not found)
- Event times out
- Event exits with non-zero status
- Concurrency limit reached
- History save fails

### Fatal Errors (prevent scheduler startup)

- Invalid cron syntax in configuration
- Configuration file parse errors
- Cannot create event history directory

## Security Considerations

- Events run with the same privileges as the BBS process
- Use absolute paths for commands to avoid PATH attacks
- Validate command paths and arguments before enabling events
- Use `timeout_seconds` to prevent runaway processes
- Review environment variables for sensitive data
- Events run non-interactively (no stdin/terminal access)

## Troubleshooting

### Event Not Running

**Check logs for scheduler startup:**

```bash
tail -f data/logs/vision3.log | grep "Event"
```

Look for:
```text
INFO: Event scheduler started with N events
INFO: Event 'event_id' scheduled: @hourly
```

**If missing:**
1. Verify `enabled: true` at root level AND event level in `configs/events.json`
2. Check cron schedule syntax
3. Review BBS startup logs for configuration errors
4. Restart BBS after config changes

### Command Not Found

```text
ERROR: Event 'binkd_poll' failed to start: fork/exec /usr/local/bin/binkd: no such file or directory
```

**Fix:**
- Find correct path: `which binkd`
- Use absolute path in `command` field
- Test command manually before adding to scheduler

### Config File Not Found

```text
ERROR: Event 'v3mail_toss' stderr: cannot open config directory
```

**Fix:**
1. Verify directory exists: `ls -la configs/`
2. Use absolute path or `{BBS_ROOT}/configs` for `--config` argument
3. Check `working_directory` is set correctly
4. Verify directory permissions

### Event Timeout

```text
ERROR: Event 'event_id' timed out after 300s
```

**Fix:**
- Increase `timeout_seconds` value
- Optimize the command/script
- Check for hung processes: `ps aux | grep command-name`
- Test command duration manually

### Event Always Skipped

```text
WARN: Event 'binkd_poll' skipped: already running
```

**Fix:**
- Event still running from previous execution
- Increase `timeout_seconds` if command is slow
- Space out schedule (use `*/30` instead of `*/15`)
- Check for stuck processes

**Max concurrency reached:**

```text
WARN: Event 'backup' skipped: max concurrent events reached (3)
```

**Fix:**
- Increase `max_concurrent_events`
- Stagger event schedules to avoid conflicts
- Reduce event durations

### Event Always Failing

```text
ERROR: Event 'event_id' failed with exit code 1
ERROR: Event 'event_id' stderr: [error message]
```

**Debug steps:**
1. Check stderr in BBS logs (DEBUG level)
2. Test command manually with exact arguments
3. Verify working directory exists and is accessible
4. Check file permissions on command and config files
5. Test environment variables are set correctly
6. Review command-specific logs if available

## Disabling the Scheduler

To disable the scheduler entirely, set `enabled: false` in `configs/events.json`:

```json
{
  "enabled": false,
  "max_concurrent_events": 3,
  "events": []
}
```

The BBS will start normally but the scheduler will not run.

## Future Enhancements

Planned features for future releases:

- **Event dependencies**: `run_after` and `delay_after_seconds` support
- **Event chains**: Automatic sequential execution
- **Manual triggers**: API/command to trigger events on-demand
- **Event output capture**: Store full output for review
- **Notification hooks**: Alert on event failures
- **Web interface**: View history and manage events via web UI

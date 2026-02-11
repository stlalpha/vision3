# Event Scheduler

The ViSiON/3 BBS includes a built-in event scheduler that can automatically execute maintenance tasks, poll FTN mail, and run other scheduled operations using cron-style syntax.

## Overview

The event scheduler provides:

- **Cron-style scheduling**: Standard cron syntax with seconds support
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

```
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

## Common Use Cases

### FTN Mail Polling (Binkd)

Poll FTN mail hubs every 15 minutes:

```json
{
  "id": "ftn_poll",
  "name": "FTN Mail Poll",
  "schedule": "*/15 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-p"],
  "working_directory": "/opt/ftn",
  "timeout_seconds": 300,
  "enabled": true,
  "environment_vars": {
    "BINKD_CONFIG": "/opt/ftn/binkd.cfg"
  }
}
```

### Echomail Tossing (HPT)

Toss echomail every hour:

```json
{
  "id": "hpt_toss",
  "name": "HPT Toss Echomail",
  "schedule": "@hourly",
  "command": "/usr/local/bin/hpt",
  "args": ["toss"],
  "working_directory": "/opt/ftn",
  "timeout_seconds": 600,
  "enabled": true
}
```

### Nightly Maintenance

Run maintenance script at 3 AM:

```json
{
  "id": "maintenance",
  "name": "Nightly Maintenance",
  "schedule": "0 3 * * *",
  "command": "/opt/vision3/scripts/maintenance.sh",
  "timeout_seconds": 3600,
  "enabled": true
}
```

### Backup

Daily backup at 2 AM:

```json
{
  "id": "backup",
  "name": "Daily Backup",
  "schedule": "0 2 * * *",
  "command": "/usr/local/bin/backup.sh",
  "args": ["{BBS_ROOT}/data", "/backups/bbs-{DATE}.tar.gz"],
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

```
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

1. Check `configs/events.json`:
   - Is `enabled: true` at root level?
   - Is the specific event `enabled: true`?
   - Is the cron schedule correct?

2. Check BBS logs for:
   - `Event scheduler started` message
   - Event scheduled confirmation
   - Warning messages about skipped events

3. Verify command:
   - Use absolute path to executable
   - Test command manually with same arguments
   - Check file permissions

### Event Timeout

- Increase `timeout_seconds` value
- Optimize the command/script
- Check for hung processes

### Event Always Failing

- Check stderr output in DEBUG logs
- Verify working directory exists and is accessible
- Test environment variables
- Ensure required files/directories exist

### Events Not Concurrent

- Check `max_concurrent_events` setting
- Look for "already running" warnings in logs
- Verify event durations aren't overlapping

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

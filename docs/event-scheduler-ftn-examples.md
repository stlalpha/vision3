# Event Scheduler - FTN Mail Examples

Common configurations for FTN (FidoNet Technology Network) mail processing with the event scheduler.

## Important Path Notes

- **{BBS_ROOT}** is automatically replaced with your BBS installation directory (e.g., `/home/bbs/git/vision3`)
- **Command paths** depend on where you installed binkd/hpt - common locations:
  - System: `/usr/local/bin/binkd`, `/usr/local/bin/hpt`
  - User install: `~/bin/binkd`, `~/bin/hpt`
  - Docker: `/usr/bin/binkd`, `/usr/bin/hpt`
- **Config paths** should match your actual FTN configuration location

## ViSiON/3 Directory Structure for FTN

```
{BBS_ROOT}/
├── data/
│   └── ftn/
│       ├── binkd.conf     # Binkd configuration
│       ├── hpt.conf       # HPT configuration
│       ├── inbound/       # Incoming mail
│       ├── outbound/      # Outgoing mail
│       └── temp/          # Temporary files
├── configs/
│   └── events.json        # This file
└── data/logs/
    └── vision3.log        # Event logs here
```

## Binkd Examples

### Simple Poll (All Configured Nodes)

```json
{
  "id": "binkd_poll_all",
  "name": "Binkd Poll All Nodes",
  "schedule": "*/30 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-p"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 300,
  "enabled": true
}
```

### Poll Specific Node

```json
{
  "id": "binkd_poll_fsxnet",
  "name": "Binkd Poll FSXNet Hub",
  "schedule": "*/15 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": [
    "-P",
    "21:4/158@fsxnet",
    "-D",
    "{BBS_ROOT}/data/ftn/binkd.conf"
  ],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 300,
  "enabled": true
}
```

### Poll Multiple Networks

```json
{
  "id": "binkd_poll_fidonet",
  "name": "Binkd Poll FidoNet",
  "schedule": "0,30 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-P", "1:123/456@fidonet", "-D", "{BBS_ROOT}/data/ftn/binkd.conf"],
  "timeout_seconds": 300,
  "enabled": true
},
{
  "id": "binkd_poll_fsxnet",
  "name": "Binkd Poll FSXNet",
  "schedule": "15,45 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-P", "21:4/158@fsxnet", "-D", "{BBS_ROOT}/data/ftn/binkd.conf"],
  "timeout_seconds": 300,
  "enabled": true
}
```

## HPT (Highly Portable Tosser) Examples

### Toss Echomail Hourly

```json
{
  "id": "hpt_toss",
  "name": "HPT Toss Echomail",
  "schedule": "@hourly",
  "command": "/usr/local/bin/hpt",
  "args": [
    "toss",
    "-c",
    "{BBS_ROOT}/data/ftn/hpt.conf"
  ],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 600,
  "enabled": true
}
```

### Scan and Pack Every 2 Hours

```json
{
  "id": "hpt_scan_pack",
  "name": "HPT Scan and Pack",
  "schedule": "0 */2 * * *",
  "command": "/usr/local/bin/hpt",
  "args": [
    "scan",
    "pack",
    "-c",
    "{BBS_ROOT}/data/ftn/hpt.conf"
  ],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 900,
  "enabled": true
}
```

### Full Mail Processing Cycle

Process mail after polling with a delay:

```json
{
  "id": "binkd_poll",
  "name": "Poll FTN Mail",
  "schedule": "*/30 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-p", "-D", "{BBS_ROOT}/data/ftn/binkd.conf"],
  "timeout_seconds": 300,
  "enabled": true
},
{
  "id": "hpt_toss_after_poll",
  "name": "Toss Mail After Poll",
  "schedule": "5,35 * * * *",
  "command": "/usr/local/bin/hpt",
  "args": ["toss", "-c", "{BBS_ROOT}/data/ftn/hpt.conf"],
  "timeout_seconds": 600,
  "enabled": true
},
{
  "id": "hpt_pack_after_toss",
  "name": "Pack Mail After Toss",
  "schedule": "10,40 * * * *",
  "command": "/usr/local/bin/hpt",
  "args": ["pack", "-c", "{BBS_ROOT}/data/ftn/hpt.conf"],
  "timeout_seconds": 600,
  "enabled": true
}
```

This creates a workflow:
- 00:00, 00:30 - Poll mail
- 00:05, 00:35 - Toss received mail
- 00:10, 00:40 - Pack outbound mail

## Finding Your Binary Paths

### Check if binkd/hpt are installed:

```bash
which binkd
which hpt
```

### Common install locations:

```bash
# System-wide
/usr/bin/binkd
/usr/bin/hpt
/usr/local/bin/binkd
/usr/local/bin/hpt

# User install
~/bin/binkd
~/bin/hpt
/opt/binkd/bin/binkd
/opt/hpt/bin/hpt
```

### Test your command manually:

```bash
# From BBS directory
cd /home/bbs/git/vision3

# Test binkd poll
/usr/local/bin/binkd -p -D data/ftn/binkd.conf

# Test HPT toss
/usr/local/bin/hpt toss -c data/ftn/hpt.conf
```

If these work manually, they'll work in the scheduler.

## Configuration File Paths

Your FTN tools need configuration files. Common locations:

### Binkd Config

```bash
# ViSiON/3 data directory (recommended)
{BBS_ROOT}/data/ftn/binkd.conf

# System-wide
/etc/binkd/binkd.conf
/usr/local/etc/binkd.conf

# User home
~/.binkd/binkd.conf
```

### HPT Config

```bash
# ViSiON/3 data directory (recommended)
{BBS_ROOT}/data/ftn/hpt.conf

# System-wide
/etc/ftn/config
/usr/local/etc/ftn/config

# User home
~/.ftn/config
```

## Logging

### View event execution:

```bash
tail -f data/logs/vision3.log | grep "Event"
```

### View binkd/hpt output:

Event output is captured and logged at DEBUG level:

```bash
grep "DEBUG: Event" data/logs/vision3.log
```

## Troubleshooting

### "Command not found"

```
ERROR: Event 'binkd_poll' failed to start: fork/exec /usr/local/bin/binkd: no such file or directory
```

**Fix:** Find the correct path with `which binkd` and update the `command` field.

### "Config file not found"

```
ERROR: Event 'hpt_toss' stderr: Cannot open config file
```

**Fix:**
1. Check if config file exists: `ls -la data/ftn/hpt.conf`
2. Use absolute path or `{BBS_ROOT}/data/ftn/hpt.conf`
3. Verify `working_directory` is set correctly

### Events not running

```
WARN: Event 'binkd_poll' skipped: already running
```

**Fix:** Event is still running from previous execution. Either:
- Increase timeout
- Space out the schedule (e.g., use `*/30` instead of `*/15`)
- Check for hung processes: `ps aux | grep binkd`

## Complete Working Example

Copy this to your `configs/events.json`:

```json
{
  "enabled": true,
  "max_concurrent_events": 3,
  "events": [
    {
      "id": "binkd_poll_fsxnet",
      "name": "Poll FSXNet Mail",
      "schedule": "*/30 * * * *",
      "command": "/usr/local/bin/binkd",
      "args": ["-p", "-D", "{BBS_ROOT}/data/ftn/binkd.conf"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 300,
      "enabled": true
    },
    {
      "id": "hpt_toss",
      "name": "Toss Echomail",
      "schedule": "5,35 * * * *",
      "command": "/usr/local/bin/hpt",
      "args": ["toss", "-c", "{BBS_ROOT}/data/ftn/hpt.conf"],
      "working_directory": "{BBS_ROOT}",
      "timeout_seconds": 600,
      "enabled": true
    }
  ]
}
```

**Remember to:**
1. Update `/usr/local/bin/binkd` and `/usr/local/bin/hpt` to your actual paths
2. Ensure config files exist at the specified paths
3. Test commands manually before enabling in scheduler
4. Restart BBS after config changes

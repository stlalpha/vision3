# FTN Echomail Setup Guide

This guide walks you through configuring FTN (FidoNet Technology Network) echomail
on your Vision/3 BBS. By the end, your board will be sending and receiving echomail
across one or more FTN networks.

## Overview

Vision/3 works with **any FTN-compatible mailer** — it does not include one built-in.
The mailer handles network transport (BinkP sessions with your hub); Vision/3's
**v3mail** commands handle everything else:

- **v3mail toss** — Unpacks ZIP bundles and tosses `.pkt` files into JAM message bases
- **v3mail scan** — Scans JAM bases for new outbound echomail and creates `.pkt` files
- **v3mail ftn-pack** — Packs outbound `.pkt` files into ZIP bundles for mailer pickup

Vision/3 releases include a pre-built **binkd** binary in `bin/` and example
configuration. Binkd is the mailer that has been tested with Vision/3, but any
mailer that speaks BinkP and uses standard inbound/outbound directory conventions
will work the same way.

- binkd source: <https://github.com/pgul/binkd>

### How It Works

```text
Your Hub <--binkd--> secure_in/ --v3mail toss--> JAM bases <-- Vision/3 --> Users
                                                       |
                               v3mail scan+ftn-pack --> out/ --binkd--> Hub
```

1. **binkd** connects to your hub and receives ZIP bundles/`.pkt` files into a secure
   inbound directory
2. **v3mail toss** unpacks bundles and tosses packets into JAM message bases
3. Vision/3 reads the JAM bases and displays messages to your users
4. When a user posts a reply, Vision/3 writes it to the JAM base
5. **v3mail scan** creates outbound `.pkt` files from new JAM messages
6. **v3mail ftn-pack** bundles the `.pkt` files into ZIP archives
7. **binkd** picks up the outbound bundles and delivers them to your hub

## Prerequisites

Before starting, you need:

- A working Vision/3 installation
- An FTN network membership (address assigned by your hub/network coordinator)
- Your hub's connection details (address, hostname/IP, port, password)
- The network's `.na` file (list of available echo areas)
- An FTN-compatible mailer — binkd is included in `bin/` and is what Vision/3 has been tested with

## Directory Structure

FTN data lives under `data/ftn/` within your Vision/3 installation. Here is the
full directory layout:

```text
vision3/
├── bin/
│   └── binkd              # binkd binary
├── configs/
│   ├── config.json        # Main BBS config (boardName used in origin lines)
│   ├── ftn.json           # FTN network configuration (tosser, links, paths)
│   ├── message_areas.json # Message area definitions
│   └── conferences.json   # Conference groupings
├── data/
│   ├── ftn/
│   │   ├── binkd.conf      # binkd configuration
│   │   ├── in/             # Unsecure inbound (rarely used)
│   │   ├── secure_in/      # Secure inbound (where binkd puts received bundles)
│   │   ├── temp_in/        # Temporary extraction dir (v3mail toss)
│   │   ├── temp_out/       # Staged outbound .pkt files (v3mail scan output)
│   │   ├── out/            # Outbound bundles (binkd picks up here)
│   │   └── dupes.json      # Dupe detection database
│   └── msgbases/
│       ├── fsx_gen.jhr    # JAM message base files (one set per area)
│       ├── fsx_gen.jdt
│       ├── fsx_gen.jdx
│       ├── fsx_gen.jlr    # Per-user lastread + v3mail export high-water mark
│       └── ...
└── helper                 # Setup utility
```

## Step-by-Step Setup

### Step 1: Create FTN Directories

Create the required directory structure:

```bash
mkdir -p data/ftn/{in,secure_in,temp_in,temp_out,out,logs}
```

### Step 2: Set Up Your FTN Mailer

A pre-built `binkd` binary is included in the Vision/3 release archive under `bin/`.
If you downloaded a release, it's already there. Any other FTN-compatible mailer
that uses standard inbound/outbound directories will work the same way.

To build binkd from source or install via package manager instead:

```bash
# From source
git clone https://github.com/pgul/binkd && cd binkd && make
cp binkd /path/to/vision3/bin/

# Or via package manager (Debian/Ubuntu)
apt install binkd
cp $(which binkd) bin/
```

### Step 3: Import Echo Areas with `helper`

The `helper` utility reads a standard `.na` file and configures Vision/3's JSON
config files automatically. This is the easiest way to set up a new network.

**Get the NA file** from your hub or network coordinator. It looks like this:

```text
FSX_GEN              General Chat + More..
FSX_BBS              BBS Support/Dev
FSX_RETRO            Retro Computing/Tech
FSX_GAMING           Games/Gaming
```

Each line has an echo tag followed by a description.

**Run `helper ftnsetup`:**

```bash
./helper ftnsetup \
  --na fsxnet.na \
  --address 21:4/158.1 \
  --hub 21:4/158 \
  --hub-password MYSECRET \
  --network fsxnet \
  --acs-write s10
```

**Options:**

| Flag                   | Required | Description                                                      |
| ---------------------- | -------- | ---------------------------------------------------------------- |
| `--na <path>`          | Yes      | Path to the `.na` file                                           |
| `--address <addr>`     | Yes      | Your FTN address (e.g., `21:4/158.1`)                            |
| `--hub <addr>`         | Yes      | Your hub's FTN address (e.g., `21:4/158`)                        |
| `--hub-password <pw>`  | No       | Packet password shared with your hub                             |
| `--hub-name <name>`    | No       | Human-readable hub label (default: `Hub <addr>`)                 |
| `--network <name>`     | No       | Network name (default: derived from NA filename)                 |
| `--conference-id <id>` | No       | Use an existing conference instead of creating one               |
| `--acs-read <acs>`     | No       | ACS string for read access                                       |
| `--acs-write <acs>`    | No       | ACS string for write access (e.g., `s10` for security level 10+) |
| `--config <dir>`       | No       | Config directory (default: `configs`)                            |
| `--dry-run`            | No       | Preview changes without writing files                            |
| `--quiet`              | No       | Suppress detailed output                                         |

**What `helper ftnsetup` does:**

1. Parses the `.na` file for echo area tags and descriptions
2. Creates a conference entry in `conferences.json` for the network (if one
   doesn't exist)
3. Adds echomail area entries to `message_areas.json` for each echo tag
4. Updates `ftn.json` with network and link configuration
5. Skips any echo tags that already exist (safe to re-run with updated NA files)

**Tip:** Use `--dry-run` first to preview what will be changed:

```bash
./helper ftnsetup --na fsxnet.na --address 21:4/158.1 --hub 21:4/158 --dry-run
```

### Step 4: Configure Your Mailer (binkd example)

The following is a binkd configuration. If you're using a different mailer, consult
its documentation — the key things to align with `ftn.json` are the inbound directory
and outbound directory paths (see the verification table in Step 6).

Create `data/ftn/binkd.conf`. Replace the example values with your own details:

```conf
# binkd.conf - Vision/3 BBS

#
# Domain - defines the FTN network and outbound directory
# Format: domain <network> <outbound_path> <zone>
#
domain fsxnet /home/bbs/vision3/data/ftn/out 21

#
# Your FTN address
#
address 21:4/158.1@fsxnet

#
# BBS information
#
sysname "My BBS Name"
location "City, State"
sysop "Your Name"
nodeinfo 9600,TCP,BINKP

#
# Connection settings
#
oblksize 4096
timeout 60
connect-timeout 0
call-delay 300
rescan-delay 60
maxservers 4
maxclients 1

#
# Logging
#
log /home/bbs/vision3/data/logs/binkd.log
loglevel 4
conlog 4

#
# Inbound directories
#
inbound /home/bbs/vision3/data/ftn/secure_in
inbound-nonsecure /home/bbs/vision3/data/ftn/in

#
# Housekeeping
#
kill-dup-partial-files
kill-old-partial-files 86400
kill-old-bsy 43200
try 2
hold 1800

#
# Listening port (if accepting inbound connections)
#
iport 24554

#
# Hub connection
# Format: node <address> <host:port> <password> <flags>
#
node 21:4/158@fsxnet your-hub-hostname:24555 MYSECRET -

#
# Additional options
#
percents
```

**Key settings to customize:**

- `domain` - Your network name, outbound path, and zone number
- `address` - Your assigned FTN address
- `sysname`, `location`, `sysop` - Your BBS details
- `inbound` - Must match `secure_inbound_path` in `ftn.json`
- `node` - Your hub's address, hostname:port, and shared password
- `iport` - Port to listen on for incoming BinkP connections (if your hub polls you)

### Step 5: Configure ftn.json

Edit `configs/ftn.json` to configure the internal tosser. The `helper ftnsetup`
command (Step 3) populates this file with network and link settings, but you should
verify the directory paths are correct for your installation:

```json
{
    "dupe_db_path": "data/ftn/dupes.json",
    "networks": {
        "fsxnet": {
            "internal_tosser_enabled": true,
            "own_address": "21:4/158.1",
            "inbound_path": "data/ftn/in",
            "secure_inbound_path": "data/ftn/secure_in",
            "outbound_path": "data/ftn/temp_out",
            "binkd_outbound_path": "data/ftn/out",
            "temp_path": "data/ftn/temp_in",
            "tearline": "",
            "links": [
                {
                    "address": "21:4/158",
                    "password": "MYSECRET",
                    "name": "Hub 21:4/158",
                    "echo_areas": ["FSX_GEN", "FSX_BBS", "FSX_RETRO", "FSX_GAMING"]
                }
            ]
        }
    }
}
```

**Key settings to customize:**

- `own_address` - Your FTN address (must match binkd.conf)
- `secure_inbound_path` - Must match binkd's `inbound` directive
- `binkd_outbound_path` - Must match binkd's `domain` outbound path
- `links[].address` - Your hub's FTN address
- `links[].password` - Packet password shared with your hub
- `links[].echo_areas` - List of echo tags this link carries (must match `echo_tag` in message_areas.json)

All paths are relative to the Vision/3 installation root (where you run the BBS from).

### Step 6: Verify Configuration

Check that all paths are consistent across your config files:

| What           | binkd.conf    | ftn.json              |
| -------------- | ------------- | --------------------- |
| Secure inbound | `inbound`     | `secure_inbound_path` |
| Outbound       | `domain` path | `binkd_outbound_path` |
| Hub password   | `node` line   | `links[].password`    |
| Your address   | `address`     | `own_address`         |

### Step 7: Initialize Message Bases

JAM message bases are created automatically the first time they are accessed, but
you can verify them with `v3mail`:

```bash
./v3mail stats --all
```

This will show statistics for all configured message areas and create any missing
base files.

### Step 8: Test the Connection

Start binkd in client mode to test your hub connection:

```bash
bin/binkd -c data/ftn/binkd.conf
```

The `-c` flag runs binkd as a client (calls out once and exits). Check
`data/logs/binkd.log` for connection results.

To manually toss any received bundles/packets:

```bash
./v3mail toss --config configs --data data
```

To scan for outbound messages and pack bundles:

```bash
./v3mail scan --config configs --data data
./v3mail ftn-pack --config configs --data data
```

## Running in Production

### Running Your FTN Mailer

For ongoing operation, run your mailer as a daemon so it can both accept incoming
connections and poll your hub. Example using binkd:

```bash
bin/binkd -D data/ftn/binkd.conf
```

Or use a systemd service:

```ini
[Unit]
Description=Binkd FTN Mailer
After=network.target

[Service]
Type=forking
ExecStart=/home/bbs/vision3/bin/binkd -D /home/bbs/vision3/data/ftn/binkd.conf
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

### Scanning and Packing Outbound Mail

When users post new echomail, `v3mail scan` creates outbound `.pkt` files from new
JAM messages, and `v3mail ftn-pack` bundles them for binkd. Use the Vision/3 event
scheduler (configured in `configs/events.json`) to run these automatically:

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

See [event-scheduler.md](../advanced/event-scheduler.md) for the full recommended FTN workflow configuration.

## Configuration Files Reference

### ftn.json

`configs/ftn.json` is the central configuration for the internal FTN tosser. It is
read by `v3mail toss`, `v3mail scan`, and `v3mail ftn-pack`.

**Fields:**

| Field                                    | Description                                                  |
| ---------------------------------------- | ------------------------------------------------------------ |
| `dupe_db_path`                           | Path to the dupe detection database (relative to BBS root)   |
| `networks.<key>.internal_tosser_enabled` | Set `true` to enable `v3mail` for this network               |
| `networks.<key>.own_address`             | Your FTN address (e.g., `21:4/158.1`)                        |
| `networks.<key>.inbound_path`            | Unsecured inbound directory                                  |
| `networks.<key>.secure_inbound_path`     | Secure inbound (where your mailer deposits received mail)    |
| `networks.<key>.outbound_path`           | Staging dir for outbound `.pkt` files (`v3mail scan` output) |
| `networks.<key>.binkd_outbound_path`     | Outbound bundles dir (your mailer picks up from here)        |
| `networks.<key>.temp_path`               | Temp dir for bundle extraction during toss                   |
| `networks.<key>.tearline`                | Optional tearline suffix (empty = use default)               |
| `networks.<key>.links[].address`         | Hub FTN address                                              |
| `networks.<key>.links[].password`        | Packet password shared with hub                              |
| `networks.<key>.links[].name`            | Human-readable hub label                                     |
| `networks.<key>.links[].echo_areas`      | Echo tags carried by this link                               |

**Example:**

```json
{
    "dupe_db_path": "data/ftn/dupes.json",
    "networks": {
        "fsxnet": {
            "internal_tosser_enabled": true,
            "own_address": "21:4/158.1",
            "inbound_path": "data/ftn/in",
            "secure_inbound_path": "data/ftn/secure_in",
            "outbound_path": "data/ftn/temp_out",
            "binkd_outbound_path": "data/ftn/out",
            "temp_path": "data/ftn/temp_in",
            "tearline": "",
            "links": [
                {
                    "address": "21:4/158",
                    "password": "MYSECRET",
                    "name": "Hub 21:4/158",
                    "echo_areas": ["FSX_GEN", "FSX_BBS", "FSX_RETRO", "FSX_GAMING"]
                }
            ]
        }
    }
}
```

### message_areas.json (Echomail Entries)

Each echomail area has these FTN-specific fields:

```json
{
    "id": 3,
    "tag": "FSX_GEN",
    "name": "General Chat + More..",
    "description": "General Chat + More..",
    "acs_read": "",
    "acs_write": "s10",
    "allow_anonymous": false,
    "real_name_only": false,
    "conference_id": 2,
    "base_path": "msgbases/fsx_gen",
    "area_type": "echomail",
    "echo_tag": "FSX_GEN",
    "origin_addr": "21:4/158.1",
    "network": "fsxnet"
}
```

| Field           | Description                                            |
| --------------- | ------------------------------------------------------ |
| `area_type`     | Must be `"echomail"` for FTN echo areas                |
| `echo_tag`      | The FTN echo tag (matches the AREA: kludge in packets) |
| `origin_addr`   | Your FTN address (appears in the Origin line)          |
| `network`       | Network key matching a key in `ftn.json` networks      |
| `base_path`     | Relative path (under `data/`) to the JAM base files    |
| `conference_id` | Groups this area under a conference for menu display   |

### conferences.json

Groups message areas for display in the BBS menu:

```json
[
    {
        "id": 1,
        "tag": "LOCAL",
        "name": "Local Areas",
        "description": "Local BBS discussion areas",
        "acs": ""
    },
    {
        "id": 2,
        "tag": "FSXNET",
        "name": "fsxNet",
        "description": "fsxNet message areas",
        "acs": ""
    }
]
```

## Adding a Second Network

To add another FTN network (e.g., AgoraNet alongside fsxNet):

1. Get the network's `.na` file from your hub
2. Run `helper ftnsetup` again with the new network details:

```bash
./helper ftnsetup \
  --na agoranet.na \
  --address 46:1/100.1 \
  --hub 46:1/100 \
  --hub-password HUBPASS \
  --network agoranet
```

3. Add the new domain, address, and node to `binkd.conf`:

```conf
domain agoranet /home/bbs/vision3/data/ftn/out 46
address 46:1/100.1@agoranet
node 46:1/100@agoranet hub-hostname:24554 HUBPASS -
```

4. Restart binkd

## Troubleshooting

### No messages arriving

- Check your mailer's log for connection errors (binkd: `data/logs/binkd.log`)
- Verify your hub's hostname, port, and password are correct
- Make sure `secure_inbound_path` in `ftn.json` matches your mailer's inbound directory
- Run your mailer in one-shot client mode manually and watch the output (binkd: `bin/binkd -c data/ftn/binkd.conf`)

### Messages arriving but not visible in BBS

- Run `./v3mail toss --config configs --data data` manually to toss pending
  bundles/packets
- Verify `echo_tag` in message_areas.json matches the AREA tag in the incoming packets
- Run `./v3mail stats --all` to verify message counts

### Outbound messages not sending

- Run `./v3mail scan --config configs --data data` to create outbound packets
- Run `./v3mail ftn-pack --config configs --data data` to bundle them
- Check that `binkd_outbound_path` in `ftn.json` matches the domain path in binkd.conf
- Verify the echo area's hub address is listed in `links[].echo_areas` in `ftn.json`

### Duplicate messages

- `v3mail toss` handles dupe detection via `data/ftn/dupes.json`
- The dupe window is configured by `DupeWindow` in the tosser (default: 30 days)

### Bad/undeliverable messages

- Check `data/msgbases/bad` (via `./v3mail stats data/msgbases/bad`) for messages
  that could not be tossed
- Usually means the AREA tag in the packet doesn't match any `echo_tag` in
  message_areas.json or any `echo_areas` entry in `ftn.json` links

## Useful Commands

```bash
# Test hub connection (one-shot client call)
bin/binkd -c data/ftn/binkd.conf

# Toss received bundles/packets into JAM bases
./v3mail toss --config configs --data data

# Scan JAM bases for new outbound messages (creates .pkt files)
./v3mail scan --config configs --data data

# Pack outbound .pkt files into ZIP bundles for binkd pickup
./v3mail ftn-pack --config configs --data data

# View message base statistics
./v3mail stats --all

# Check a specific area
./v3mail stats data/msgbases/fsx_gen

# Fix corrupted message base
./v3mail fix data/msgbases/fsx_gen

# Purge old messages (90 days)
./v3mail purge --days 90 --all

# Pack/defragment message bases
./v3mail pack --all

# Preview helper changes before applying
./helper ftnsetup --na network.na --address 21:4/158.1 --hub 21:4/158 --dry-run
```

# FTN Echomail Setup Guide

This guide walks you through configuring FTN (FidoNet Technology Network) echomail
on your Vision/3 BBS. By the end, your board will be sending and receiving echomail
across one or more FTN networks.

## Overview

Vision/3 uses **external binaries** for mail transport and tossing:

- **binkd** - FTN mailer (sends/receives packets over BinkP protocol)
  - GitHub: <https://github.com/pgul/binkd>
- **hpt** (Husky Project Tosser) - Tosses incoming packets into JAM message bases
  and scans outbound messages into packets
  - GitHub: <https://github.com/huskyproject>

Vision/3 itself manages the JAM message bases, presents messages to users, and
handles new message creation. The external tools handle the network side.

### How It Works

```text
Your Hub <--binkd--> secure_in/ --hpt toss--> JAM bases <-- Vision/3 --> Users
                                                  |
                                         hpt scan --> outbound/ --binkd--> Hub
```

1. **binkd** connects to your hub and receives `.pkt` files into a secure inbound
   directory
2. binkd triggers **hpt** to toss those packets into JAM message bases
3. Vision/3 reads the JAM bases and displays messages to your users
4. When a user posts a reply, Vision/3 writes it to the JAM base
5. **hpt** scans the JAM bases for new outbound messages and creates `.pkt` files
6. **binkd** picks up the outbound packets and delivers them to your hub

## Prerequisites

Before starting, you need:

- A working Vision/3 installation
- An FTN network membership (address assigned by your hub/network coordinator)
- Your hub's connection details (address, hostname/IP, port, password)
- The network's `.na` file (list of available echo areas)
- **binkd** and **hpt** binaries installed (see [Installing External Binaries](#step-2-install-external-binaries))

## Directory Structure

FTN data lives under `data/ftn/` within your Vision/3 installation. Here is the
full directory layout:

```text
vision3/
├── bin/
│   ├── binkd              # binkd binary
│   └── hpt                # hpt binary
├── configs/
│   ├── config.json        # Main BBS config (boardName used in origin lines)
│   ├── ftn.json           # FTN network configuration
│   ├── message_areas.json # Message area definitions
│   └── conferences.json   # Conference groupings
├── data/
│   ├── ftn/
│   │   ├── binkd.conf     # binkd configuration
│   │   ├── husky.cfg      # hpt/Husky configuration
│   │   ├── in/            # Unsecure inbound (rarely used)
│   │   ├── secure_in/     # Secure inbound (where binkd puts received packets)
│   │   ├── temp_in/       # Temporary inbound
│   │   ├── temp_out/      # Temporary outbound
│   │   ├── out/           # Outbound (binkd picks up packets here)
│   │   ├── logs/          # binkd and hpt log files
│   │   ├── dupehist/      # hpt dupe history database
│   │   └── dloads/        # File echos (if applicable)
│   └── msgbases/
│       ├── fsx_gen.jhr    # JAM message base files (one set per area)
│       ├── fsx_gen.jdt
│       ├── fsx_gen.jdx
│       ├── fsx_gen.jlr
│       └── ...
└── helper                 # Setup utility
```

## Step-by-Step Setup

### Step 1: Create FTN Directories

Create the required directory structure:

```bash
mkdir -p data/ftn/{in,secure_in,temp_in,temp_out,out,logs,dupehist,dloads}
```

### Step 2: Install External Binaries

Place `binkd` and `hpt` in the `bin/` directory. You can obtain these from:

- **binkd**: Build from source at <https://github.com/pgul/binkd> or install via
  your distribution's package manager (`apt install binkd`, etc.)
- **hpt**: Part of the Husky FidoNet software project. Build from source at
  <https://github.com/huskyproject> or install via package manager if available.

```bash
mkdir -p bin
cp /path/to/binkd bin/
cp /path/to/hpt bin/
chmod +x bin/binkd bin/hpt
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

### Step 4: Configure binkd

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
log /home/bbs/vision3/data/ftn/logs/binkd.log
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
# After receiving files, run hpt to toss packets into message bases
#
prescan
exec "/home/bbs/vision3/bin/hpt -c /home/bbs/vision3/data/ftn/husky.cfg toss" *.[mwtfsMWTFS][oehrauOEHRAU][0-9a-zA-Z] *.pkt

#
# Additional options
#
percents
```

**Key settings to customize:**

- `domain` - Your network name, outbound path, and zone number
- `address` - Your assigned FTN address
- `sysname`, `location`, `sysop` - Your BBS details
- `inbound` - Must match where hpt expects to find incoming packets
- `node` - Your hub's address, hostname:port, and shared password
- `exec` - Full path to hpt and husky.cfg on your system
- `iport` - Port to listen on for incoming BinkP connections (if your hub polls you)

### Step 5: Configure hpt (Husky)

Create `data/ftn/husky.cfg`. This tells hpt where to find packets and where to
store messages:

```conf
# Husky Configuration File

#
# Your FTN address
#
Address 21:4/158.1

#
# Directory paths (use absolute paths)
#
DupeHistoryDir /home/bbs/vision3/data/ftn/dupehist
EchoTossLog /home/bbs/vision3/data/ftn/logs/echotoss.log
FileAreaBaseDir /home/bbs/vision3/data/ftn/dloads
LogFileDir /home/bbs/vision3/data/ftn/logs
MsgBaseDir /home/bbs/vision3/data/msgbases
Outbound /home/bbs/vision3/data/ftn/out
PassFileAreaDir /home/bbs/vision3/data/ftn/dloads/pass
ProtInbound /home/bbs/vision3/data/ftn/secure_in
TempInbound /home/bbs/vision3/data/ftn/temp_in
TempOutbound /home/bbs/vision3/data/ftn/temp_out
inbound /home/bbs/vision3/data/ftn/in

#
# Dupe detection - reject messages seen within this many days
#
areasmaxdupeage 30

#
# Archive tools for bundled mail
#
Unpack "/usr/bin/unzip -j -Loqq $a -d $p" 0 504b0304
Pack zip /usr/bin/zip -9 -j -q $a $f

#
# Link definition - your hub
#
Link 21:4/158
Aka 21:4/158.0
ouraka 21:4/158.1
Packer Zip
echomailFlavour Crash

#
# Routing - send all mail for this zone through your hub
#
route crash 21:4/158 21:*

#
# Netmail area
#
NetmailArea NETMAIL /home/bbs/vision3/data/msgbases/netmail -b Jam

#
# Bad and dupe areas (messages that can't be delivered or are duplicates)
#
BadArea BadArea /home/bbs/vision3/data/msgbases/bad -b Jam
DupeArea DupeArea /home/bbs/vision3/data/msgbases/dupe -b Jam

#
# Echo areas
# Format: EchoArea <TAG> <base_path> -a <your_address> -b Jam <hub_address>
#
EchoArea FSX_GEN /home/bbs/vision3/data/msgbases/fsx_gen -a 21:4/158.1 -b Jam 21:4/158
EchoArea FSX_BBS /home/bbs/vision3/data/msgbases/fsx_bbs -a 21:4/158.1 -b Jam 21:4/158
EchoArea FSX_RETRO /home/bbs/vision3/data/msgbases/fsx_retro -a 21:4/158.1 -b Jam 21:4/158
EchoArea FSX_GAMING /home/bbs/vision3/data/msgbases/fsx_gaming -a 21:4/158.1 -b Jam 21:4/158
```

**Key settings to customize:**

- `Address` - Your FTN address (must match binkd.conf)
- All directory paths - Use absolute paths to your Vision/3 installation
- `Link` / `Aka` / `ouraka` - Your hub's address and your own address
- `ProtInbound` - Must match binkd's `inbound` directive
- `Outbound` - Must match binkd's `domain` outbound path
- `EchoArea` lines - One per echo area, path matches `base_path` in
  `message_areas.json` (prefixed with your data directory)

**Important:** The `EchoArea` base paths must point to the same JAM files that
Vision/3 uses. If `message_areas.json` has `"base_path": "msgbases/fsx_gen"`, then
hpt needs `/full/path/to/data/msgbases/fsx_gen`. The `-b Jam` flag tells hpt to
use JAM format (required by Vision/3).

### Step 6: Verify Configuration

Check that all paths are consistent across your config files:

| What           | binkd.conf    | husky.cfg        | ftn.json                   |
| -------------- | ------------- | ---------------- | -------------------------- |
| Secure inbound | `inbound`     | `ProtInbound`    | `inbound_path` (internal)  |
| Outbound       | `domain` path | `Outbound`       | `outbound_path` (internal) |
| Message bases  | —             | `EchoArea` paths | —                          |
| Hub password   | `node` line   | —                | `links[].password`         |
| Your address   | `address`     | `Address`        | `own_address`              |

### Step 7: Initialize Message Bases

JAM message bases are created automatically the first time they are accessed, but
you can verify them with `jamutil`:

```bash
./jamutil stats --all
```

This will show statistics for all configured message areas and create any missing
base files.

### Step 8: Test the Connection

Start binkd in client mode to test your hub connection:

```bash
bin/binkd -c data/ftn/binkd.conf
```

The `-c` flag runs binkd as a client (calls out once and exits). Check
`data/ftn/logs/binkd.log` for connection results.

To manually toss any received packets:

```bash
bin/hpt -c data/ftn/husky.cfg toss
```

To scan for outbound messages:

```bash
bin/hpt -c data/ftn/husky.cfg scan
```

## Running in Production

### Running binkd as a Daemon

For ongoing operation, run binkd as a daemon so it can both accept incoming
connections and poll your hub:

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

### Scanning for Outbound Mail

When users post new echomail, hpt needs to scan the JAM bases and create outbound
packets. Set up a cron job to do this periodically:

```cron
# Scan for outbound echomail every 5 minutes
*/5 * * * * /home/bbs/vision3/bin/hpt -c /home/bbs/vision3/data/ftn/husky.cfg scan
```

Alternatively, you can call hpt scan from a script that runs alongside Vision/3.

## Configuration Files Reference

### ftn.json

This is Vision/3's internal FTN configuration. It is managed by the `helper`
tool but can be edited manually.

```json
{
    "dupe_db_path": "data/ftn/dupes.json",
    "networks": {
        "fsxnet": {
            "enabled": true,
            "own_address": "21:4/158.1",
            "inbound_path": "data/ftn/fsxnet/inbound",
            "outbound_path": "data/ftn/fsxnet/outbound",
            "temp_path": "data/ftn/fsxnet/temp",
            "poll_interval_seconds": 300,
            "links": [
                {
                    "address": "21:4/158",
                    "password": "MYSECRET",
                    "name": "Hub 21:4/158",
                    "echo_areas": [
                        "FSX_GEN",
                        "FSX_BBS"
                    ]
                }
            ]
        }
    }
}
```

| Field                                  | Description                                                  |
| -------------------------------------- | ------------------------------------------------------------ |
| `dupe_db_path`                         | Path to the dupe detection database (shared across networks) |
| `networks.<key>.enabled`               | Enable/disable this network                                  |
| `networks.<key>.own_address`           | Your FTN address on this network                             |
| `networks.<key>.inbound_path`          | Inbound packet directory                                     |
| `networks.<key>.outbound_path`         | Outbound packet directory                                    |
| `networks.<key>.temp_path`             | Temp directory for failed packets                            |
| `networks.<key>.poll_interval_seconds` | Internal poll interval (seconds)                             |
| `networks.<key>.links[].address`       | Hub/link FTN address                                         |
| `networks.<key>.links[].password`      | Packet password                                              |
| `networks.<key>.links[].name`          | Human-readable link name                                     |
| `networks.<key>.links[].echo_areas`    | Echo tags routed to this link                                |

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

1. Add the new domain, address, and node to `binkd.conf`:

```conf
domain agoranet /home/bbs/vision3/data/ftn/out 46
address 46:1/100.1@agoranet
node 46:1/100@agoranet hub-hostname:24554 HUBPASS -
```

1. Add the link and echo areas to `husky.cfg`:

```conf
Link 46:1/100
Aka 46:1/100.0
ouraka 46:1/100.1
Packer Zip
echomailFlavour Crash

route crash 46:1/100 46:*

EchoArea AGN_GEN /home/bbs/vision3/data/msgbases/agn_gen -a 46:1/100.1 -b Jam 46:1/100
```

1. Restart binkd

## Troubleshooting

### No messages arriving

- Check `data/ftn/logs/binkd.log` for connection errors
- Verify your hub's hostname, port, and password are correct
- Make sure `secure_in/` matches between binkd.conf (`inbound`) and husky.cfg
  (`ProtInbound`)
- Run `bin/binkd -c data/ftn/binkd.conf` manually and watch the output

### Messages arriving but not visible in BBS

- Run `bin/hpt -c data/ftn/husky.cfg toss` manually to toss pending packets
- Check `data/ftn/logs/` for hpt error output
- Verify `EchoArea` paths in husky.cfg match `base_path` in message_areas.json
  (husky.cfg uses absolute paths, message_areas.json uses relative paths under
  `data/`)
- Run `./jamutil stats --all` to verify message counts

### Outbound messages not sending

- Run `bin/hpt -c data/ftn/husky.cfg scan` to create outbound packets
- Check that `Outbound` in husky.cfg matches the domain path in binkd.conf
- Verify the echo area's hub address is listed at the end of the `EchoArea` line
  in husky.cfg

### Duplicate messages

- hpt handles dupe detection via `DupeHistoryDir` — make sure the directory exists
  and is writable
- Increase `areasmaxdupeage` in husky.cfg if you're seeing old dupes reappear

### Bad/undeliverable messages

- Check `data/msgbases/bad` (via `./jamutil stats data/msgbases/bad`) for messages
  that hpt couldn't route
- Usually means the echo tag doesn't have a matching `EchoArea` line in husky.cfg

## Useful Commands

```bash
# Test hub connection (one-shot client call)
bin/binkd -c data/ftn/binkd.conf

# Toss received packets into JAM bases
bin/hpt -c data/ftn/husky.cfg toss

# Scan JAM bases for outbound messages
bin/hpt -c data/ftn/husky.cfg scan

# View message base statistics
./jamutil stats --all

# Check a specific area
./jamutil stats data/msgbases/fsx_gen

# Fix corrupted message base
./jamutil fix data/msgbases/fsx_gen

# Purge old messages (90 days)
./jamutil purge --days 90 --all

# Pack/defragment message bases
./jamutil pack --all

# Preview helper changes before applying
./helper ftnsetup --na network.na --address 21:4/158.1 --hub 21:4/158 --dry-run
```

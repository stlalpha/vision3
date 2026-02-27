# v3mail — Vision3 Mail Utility

`v3mail` is the built-in command-line tool for managing JAM message bases and FTN echomail on a ViSiON/3 BBS. It combines message base maintenance (stats, defragmentation, purging, integrity checks, reply-thread linking) with a complete FTN echomail tosser/scanner/packer that replaces external tools like HPT/Husky.

## Commands

### JAM Base Commands

| Command    | Description                                                                  |
| ---------- | ---------------------------------------------------------------------------- |
| `stats`    | Display message counts, base sizes, and metadata for one or all areas        |
| `pack`     | Defragment a base — physically removes deleted messages and compacts storage |
| `purge`    | Delete messages exceeding per-area `max_messages` or `max_age` limits        |
| `fix`      | Verify base integrity; use `--repair` to automatically fix corrupt headers   |
| `link`     | Build reply-thread chains (`ReplyTo` / `Reply1st` / `ReplyNext` JAM fields)  |
| `lastread` | Show or reset per-user lastread pointers                                     |

### FTN Echomail Commands

| Command    | Description                                                                |
| ---------- | -------------------------------------------------------------------------- |
| `toss`     | Unpack inbound ZIP bundles and toss `.pkt` files into JAM message bases    |
| `scan`     | Scan JAM bases for new outbound echomail and create staging `.pkt` files   |
| `ftn-pack` | Pack staged `.pkt` files into ZIP bundles for binkd; writes BSO flow files |

### AreaFix Commands (via `helper`)

AreaFix requests are sent as netmail to the hub's `AreaFix` robot using the `helper areafix` command (not via `v3mail` directly).

| Option            | Description                                                                 |
| ----------------- | --------------------------------------------------------------------------- |
| `--network NAME`  | FTN network name (required)                                                 |
| `--command CMD`   | AreaFix command to send (e.g. `%LIST`, `+LINUX`, `-LINUX`)                  |
| `--seed`          | Subscribe to all areas in `message_areas.json` for this network             |
| `--seed-messages` | Number of old messages to rescan when using `--seed` (default: 25)          |
| `--link ADDR`     | Hub address to target (default: first link of the network)                  |

```bash
# List available echo areas from the hub
./helper areafix --network fsxnet --command "%LIST"

# Subscribe to a specific area
./helper areafix --network fsxnet --command "+LINUX"

# Seed all subscribed areas with the last 50 messages
./helper areafix --network fsxnet --seed --seed-messages 50
```

The `areafix_password` field on the link config is used as the netmail subject (the AreaFix password). The resulting `.pkt` is written to `outbound_path` and picked up by the next `v3mail ftn-pack` + binkd run.

## Global Options

```text
--all           Operate on all areas defined in configs/message_areas.json
--config DIR    Path to config directory (default: configs)
--data DIR      Path to data directory (default: data)
-q              Quiet mode — suppress informational output
```

## FTN-Specific Options

```text
--network NAME  Restrict toss/scan/ftn-pack to a single FTN network name
                (default: all networks with internal_tosser_enabled: true)
```

## Usage Examples

```bash
# JAM base maintenance
./v3mail stats --all
./v3mail pack --all
./v3mail purge --all
./v3mail fix --repair --all
./v3mail link --all

# Operate on a single area
./v3mail stats data/msgbases/fsx_gen
./v3mail purge data/msgbases/fsx_gen

# FTN echomail workflow
./v3mail toss --config configs --data data
./v3mail scan --config configs --data data
./v3mail ftn-pack --config configs --data data

# Limit to one network
./v3mail toss --network fsxnet
```

## FTN Configuration

FTN behaviour is controlled by `configs/ftn.json`. Global fields (top-level):

| Field                 | Description                                                     |
| --------------------- | --------------------------------------------------------------- |
| `inbound_path`        | Directory binkd delivers inbound bundles to                     |
| `secure_inbound_path` | Directory for password-authenticated sessions                   |
| `outbound_path`       | Staging directory for outbound `.pkt` files (`scan` output)     |
| `binkd_outbound_path` | binkd BSO outbound directory (`ftn-pack` output)                |
| `temp_path`           | Temporary directory for bundle extraction                       |
| `bad_area_tag`        | JAM area tag for messages with unknown echo tags (e.g. `"BAD"`) |
| `dupe_area_tag`       | JAM area tag for duplicate MSGIDs (e.g. `"DUPE"`)               |

Per-network fields (`networks.<key>`):

| Field                       | Description                                                     |
| --------------------------- | --------------------------------------------------------------- |
| `own_address`               | This node's FTN address (zone:net/node.point)                   |
| `internal_tosser_enabled`   | Set `true` to enable `v3mail` for this network                  |
| `poll_interval_seconds`     | Auto-poll interval; `0` = manual only                           |
| `tearline`                  | Custom tearline text (empty = use default)                      |

Per-link fields (`networks.<key>.links[]`):

| Field                  | Description                                                     |
| ---------------------- | --------------------------------------------------------------- |
| `address`              | Link's FTN address                                              |
| `packet_password`      | Packet password shared with hub (empty for no auth)             |
| `areafix_password`     | Password for AreaFix netmail (subject line; set by hub)         |
| `name`                 | Human-readable label for this link                              |
| `flavour`              | Delivery mode: `Normal` (default), `Crash`, `Hold`, `Direct`    |

## How Echomail Flow Works

```text
Inbound:
  binkd → secure_in/ → v3mail toss → JAM bases → Vision/3 users

Outbound:
  Users post → JAM bases → v3mail scan → temp_out/*.pkt
                         → v3mail ftn-pack → out/NNNNFFFF.DOW0 (+ .clo)
                         → binkd picks up → Hub
```

1. binkd receives a bundle from your hub and places it in `inbound_path`
2. `v3mail toss` extracts the bundle, parses each `.pkt`, and writes messages into the correct JAM bases; updates SEEN-BY and PATH; detects duplicates via `data/ftn/dupes.json`
3. Users read and reply to messages in Vision/3
4. `v3mail scan` reads new messages from JAM bases (using a per-base high-water mark stored in each area's `.jlr` file under the `v3mail` scanner user) and creates outbound `.pkt` files in `outbound_path`
5. `v3mail ftn-pack` bundles the `.pkt` files into BSO ZIP archives in `binkd_outbound_path`; if link `flavour` is `Crash`, a `.clo` flow file is written to trigger an immediate binkd call
6. binkd transmits the bundle to the hub

## Nightly Maintenance Sequence

The recommended nightly sequence (configured via the event scheduler):

| Time  | Command                     | Purpose                               |
| ----- | --------------------------- | ------------------------------------- |
| 02:00 | `v3mail fix --repair --all` | Check and repair JAM base integrity   |
| 02:15 | `v3mail purge --all`        | Remove messages past age/count limits |
| 02:30 | `v3mail pack --all`         | Defragment and compact all bases      |

## Event Scheduler Integration

`v3mail` is designed to run as scheduled events. See `configs/events.json` and [Event Scheduler](../advanced/event-scheduler.md) for full examples. A typical configuration:

```json
{
  "id": "v3mail_toss",
  "command": "{BBS_ROOT}/v3mail",
  "args": ["toss", "--config", "{BBS_ROOT}/configs", "--data", "{BBS_ROOT}/data"],
  "schedule": "@hourly"
}
```

## Troubleshooting

**Messages not appearing after toss**
Run `v3mail toss` manually and check stdout for `WARN` or `ERROR` lines. Verify `configs/ftn.json` has the correct `inbound_path` and that the echo area tag in the bundle matches an area in `configs/message_areas.json`.

**Outbound mail not leaving**
Run `v3mail scan` then `v3mail ftn-pack` manually. Check that `binkd_outbound_path` has a `.zip` bundle and (for Crash links) a `.clo` flow file. Verify binkd is configured to watch that directory.

**Messages landing in BAD area**
The echo area tag in the inbound packet does not match any area in `configs/message_areas.json`. Either add the area or update `bad_area_tag` in `configs/ftn.json` to route these to a catchall area.

**High duplicate rate**
Check `data/ftn/dupes.json`. If the file is corrupt or very large, remove it and let `v3mail toss` recreate it. Messages since the last toss will pass through on the next run.

## See Also

- [FTN Echomail](ftn-echomail.md) — End-to-end FTN setup guide
- [JAM Echomail](jam-echomail.md) — JAM message base internals
- [Event Scheduler](../advanced/event-scheduler.md) — Scheduling v3mail commands
- [Message Areas](message-areas.md) — Configuring message areas

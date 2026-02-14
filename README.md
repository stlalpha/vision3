
# ViSiON/3 BBS Software

![ViSiON/3](ViSiON3.png)

## What Is This

A modern resurrection of the classic ViSiON/2 BBS software, by people who were around and involved when it was really cool.

We're rebuilding the BBS experience the way it should have evolved — if the internet hadn't come along and ruined everything.

## What Works

- SSH server with PTY support via libssh CGO wrapper (full SyncTerm and retro terminal compatibility, modern algorithms, SSH-authenticated users skip the login screen)
- Telnet server (because who doesn't want to telnet into their BBS insecurely in 2026?)
- User signup, authentication (bcrypt), and persistence
- Menu system loading & execution (`.MNU`, `.CFG`, `.ANS` files)
- Access Control System (ACS) evaluation
- Menu password protection
- Message areas: JAM format (echomail/netmail ready), conferences, full-screen reader with scrolling/lightbar menu, 14 customizable header styles, thread searching, message list view, replies with quoting, full-screen editor, newscan, last read tracking
- Private mail: user-to-user messaging with MSG_PRIVATE flag
- FTN/echomail: FidoNet packet handling, tosser with import/export, dupe checking, echomail routing
- File areas (list files, list areas, select area)
- File downloads via ZMODEM (`sz`)
- User stats, last callers, user listing
- One-liner system
- Door/external program support with dropfile generation
- SysOp tools: user validation, ban, delete, admin user browser
- Call history tracking
- Event scheduler: cron-style task scheduler for automated maintenance, FTN mail polling, and periodic operations
- Connection security: per-IP rate limiting, max nodes, IP allowlist/blocklist with live reload

### Still Cooking

- File upload via ZMODEM (`rz` integration)
- Full file base implementation (tagging, batch downloads, upload processing)
- SysOp TUI tools (system configuration editor)


## Installation

### Docker (Recommended)

```bash
git clone https://github.com/stlalpha/vision3.git
cd vision3
docker-compose up -d
```

See [Docker Deployment Guide](documentation/docker-deployment.md) for details.

### Manual

**Requires libssh:**

```bash
# Ubuntu/Debian
sudo apt-get install libssh-dev

# Fedora/RHEL
sudo dnf install libssh-devel

# macOS
brew install libssh
```

**Quick setup:**

```bash
git clone https://github.com/stlalpha/vision3.git
cd vision3
./setup.sh                    # generates keys, copies configs, builds
nano configs/config.json      # edit your BBS settings
./build.sh                    # build it
./bin/vision3                 # fire it up
```

Listens on port 2222 by default. Default login is `felonius` / `password` — change it after first login.

## Terminal Clients

You can connect with `ssh felonius@localhost -p 2222` and it'll work, but you're going to see broken box characters, wrong colors, and none of the ANSI art will look right. Standard SSH clients don't speak CP437 — they assume UTF-8, which mangles everything the BBS is sending.

Use a proper BBS terminal instead:

- **[SyncTerm](https://syncterm.bbsdev.net/)** — the standard. Native CP437, auto-detects terminal size, handles ANSI/VT100 correctly
- **[NetRunner](https://mysticbbs.com/downloads.html)** — modern alternative with tabbed sessions and built-in phonebook

These clients handle the character encoding natively so the ANSI art, box drawing, and color schemes render the way they're supposed to.

## Documentation

- [Docker Deployment Guide](documentation/docker-deployment.md)
- [Configuration Guide](documentation/configuration.md)
- [Security Guide](documentation/security.md)
- [FTN/Echomail Setup](documentation/ftn-echomail-setup.md)
- [Event Scheduler](documentation/event-scheduler.md)
- [Developer Guide](documentation/developer-guide.md)

## Need Help?

Something broken? Confused? Just want to yell about something? Join the Discord!

- **Discord:** [discord.gg/VkjRN2Ms](https://discord.gg/VkjRN2Ms)
- **Issue Tracker:** [stlalpha/vision3/issues](https://github.com/stlalpha/vision3/issues)

## Get Involved

### Contributors

We need more hands — Go code, door games, utilities, sysop tools, anything that makes the ecosystem better. If you can wrangle a Go codebase while arguing about why HSLINK was underrated, come help.

**What we won't do:**

- Rewrite this in Rust/JavaScript/whatever
- Add a REST API and React frontend
- Turn it into a web app
- Modernize away what makes it a BBS

If this sounds like your particular flavor of madness, hit the Discord or submit a PR.

### ANSI Artists

Are you an old school ANSi artist (are you younger and infatuated for some reason with that time-period and style)? Do you need one more goddamn thing to do? Consider spending valuable free-time, compensated by nothing more than unyielding appreciation and thanks from the people that enjoy this kind of thing. There's at least 12 of us!

**What we need:**

- **Menu screens** - Main, Message, File, Door menus with that classic warez BBS aesthetic
- **Login/Logoff screens** - Welcome screens, new user applications, goodbye screens
- **Headers and prompts** - Message headers, file listings, user stats displays
- **Transition screens** - Loading screens, pause prompts, error messages
- **Special effects** - Matrix rain, plasma effects, classic BBS animations

**Style we're after:**

- Authentic early 90s underground/warez BBS aesthetic
- Classic color schemes (cyan/magenta highlights, ice colors)
- Scene-style fonts and logos
- Period-appropriate group shoutouts and "greetz"

If you have skills, old .ANS files gathering dust, or just love the aesthetic, hit us up on Discord or submit a PR.

### Beta Sysops

We need people who will actually run this thing, break it, and tell us what happened. Set up a board, invite your friends, try the weird edge cases, file issues. The best bug reports come from people running it for real.

### Discord

Come hang out: [discord.gg/VkjRN2Ms](https://discord.gg/VkjRN2Ms)

## Special Thanks

To **Crimson Blade**, who created ViSiON/2 and built something that mattered to a lot of people who didn't know yet how much they'd miss it.

Greetz to all the old school scene users, sysops, ANSI artists, door game authors, mod/utility authors, and BBS software developers who made it all worth dialing into. You built something amazing and some of us never got over it.

For the original ViSiON/2 BBS (Turbo Pascal), see: [vision-2-bbs](https://github.com/stlalpha/vision-2-bbs)

---

*Built with love by people smart enough to know better and old enough to care anyway.*

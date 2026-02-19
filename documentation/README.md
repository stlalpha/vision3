# ViSiON/3 Documentation

This directory contains comprehensive documentation for the ViSiON/3 BBS system.

## Documentation Overview

### Setup & Configuration

- [Installation Guide](installation.md) - Step-by-step installation instructions
- [Configuration Guide](configuration.md) - Detailed configuration options
- [Security Guide](security.md) - Connection security, IP filtering, and best practices
- [Menu System Guide](menu-system.md) - Understanding and customizing menus

### Development

- [Architecture Overview](architecture.md) - System design and structure
- [API Reference](api-reference.md) - Package and interface documentation
- [Developer Guide](developer-guide.md) - Contributing and extending ViSiON/3

### Operations

- [User Management](user-management.md) - Managing users and access levels
- [Message Areas](message-areas.md) - Setting up and managing message bases
- [File Areas](file-areas.md) - Configuring file areas and transfers
- [Door Programs](doors.md) - Setting up external door programs

### Networking

- [SSH Server Migration](ssh-server-migration.md) - SSH server libssh implementation details
- [Telnet Server](telnet-server.md) - Telnet server protocol and implementation details

### Reference

- [Message Header Placeholders](message-header-placeholders.md) - `MSGHDR.*` template substitutions
- [Message Reader Scrolling](message-reader-scrolling-implementation.md) - Message reader scrolling behavior
- [Message Reader Comparison](message-reader-comparison.md) - Vision-2/Pascal comparison notes
- [JAM Echomail](jam-echomail.md) - Echomail support in JAM bases

### Planning Documents

- [Message Reader Plan](message_reader_plan.md) - Message system implementation plan
- [File Transfer Plan](file_transfer_plan.md) - File transfer implementation plan

## Quick Links

- **Getting Started**: Start with the [Installation Guide](installation.md)
- **For Developers**: See [Developer Guide](developer-guide.md) and [API Reference](api-reference.md)
- **For SysOps**: Check [Configuration Guide](configuration.md), [Security Guide](security.md), and operational guides

## Last Callers Quick Reference

Use menu command:

- `RUN:LASTCALLERS` (default 10)
- `RUN:LASTCALLERS 25` (explicit count)

Template files (menu set):

- `menus/v3/templates/LASTCALL.TOP`
- `menus/v3/templates/LASTCALL.MID`
- `menus/v3/templates/LASTCALL.BOT`

Common Last Callers tokens:

- `@LO@` / `@LT@` - Logon/logoff time
- `@UN@` - User handle
- `@NOTE@` - User note (private note)
- `@ND@` - Node number
- `@CA@` - Caller number
- `@TO@` - Minutes online
- `@UC@` - Total registered users (also `@USERCT@`)

Width formatting is supported with `:n` (example: `@UN:20@`, `@NOTE:14@`).

For full details, see [Menu System Guide](menu-system.md#last-callers-runlastcallers).

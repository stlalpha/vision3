# Third-Party Software Notices

ViSiON/3 BBS bundles external utilities licensed under their own terms. All bundled software is licensed under **GNU GPL v2.0** unless otherwise noted.

**Last Updated:** February 23, 2026

---

## Bundled Utilities


### 1. Binkd

**Component:** Binkd mailer daemon  
**Version:** 1.1a-111 | **License:** GPL-2.0  
**Repository:** <https://github.com/pgul/binkd>

FidoNet mailer daemon for TCP/IP networks.

```text
Copyright (C) Pavel Gulchouck and Binkd contributors

This program is free software; you can redistribute it and/or modify it under
the terms of the GNU General Public License v2.0. See the full license at:
https://github.com/pgul/binkd/blob/master/COPYING
```

---

### 4. SEXYZ

**Component:** X/Y/Z-modem file transfer protocols  
**Version:** Current from Synchronet | **License:** GPL-2.0  
**Repository:** <https://gitlab.synchro.net/main/sbbs>

Industry-standard modem protocol utilities for reliable file transfers.

```text
Copyright (C) Rob Swindell and Synchronet contributors

This program is free software; you can redistribute it and/or modify it under
the terms of the GNU General Public License v2.0. See the full license at:
https://gitlab.synchro.net/main/sbbs/-/blob/master/LICENSE
```

---

## GPL Compliance

### Distribution Methods

- **Pre-built binaries** for convenience
- **Build scripts** to compile from source
- **Source links** to original repositories

### Requirements for Redistributors

If you redistribute Retrograde BBS with bundled GPL software:

1. Include this THIRD_PARTY_NOTICES.md file
2. Ensure source code access via original repositories
3. Preserve all license files and copyright notices
4. Document any modifications made

### Our Modifications

- **Build system only:** Modified Makefiles for cross-compilation
- **Configuration files:** Example configs with Retrograde-specific paths
- **No source changes:** Original utility code is unmodified

---

## Support & Information

### Check Versions

```bash
./binkd -?           # Binkd
./sexyz -?           # SEXYZ
```

---

**Disclaimer:** Bundled third-party software is provided by their respective authors. ViSiON/3 includes these tools for convenience but does not warranty or support them beyond basic integration.

*This document is part of the ViSiON/3 BBS project and is updated as dependencies change.*
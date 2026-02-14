# Template File Encoding Issues

The following template files have encoding corruption (UTF-8 replacement characters
where CP437/ANSI bytes should be):

- menus/v3/templates/DOORLIST.BOT
- menus/v3/templates/DOORLIST.TOP  
- menus/v3/templates/ONELINER.TOP (partially corrupted)

## Issue
Files contain ef bf bd (U+FFFD replacement character) instead of proper ANSI escape
sequences and CP437 characters.

## Resolution
These files need to be regenerated from source or replaced with properly encoded versions.
The corruption likely occurred during a UTF-8 conversion or git operation.

## Temporary Workaround
The system will still function, but door listings and one-liner displays may show
replacement characters (�) instead of intended box drawing or ANSI art.

## Action Required

### Option 1: Recreate Templates (Recommended)
Use an ANSI editor to recreate the templates with proper CP437 encoding:
- **Tools**: TheDraw, Moebius, PabloDraw, or SyncTERM's built-in editor
- **Format**: Save as raw ANSI (.ANS or plain text with ANSI codes)
- **Encoding**: CP437 (DOS/OEM)

### Option 2: Restore from Backup
If original versions exist:
```bash
git checkout <commit-hash> -- menus/v3/templates/DOORLIST.BOT
git checkout <commit-hash> -- menus/v3/templates/DOORLIST.TOP
```

### Option 3: Disable Affected Templates
If templates are not critical, they can be left as-is or removed:
```bash
# System will function with degraded display
# Users will see � characters instead of box drawing
```

## Prevention

Add to `.gitattributes` to prevent future corruption:
```gitattributes
*.ANS binary
*.MID binary
*.TOP binary
*.BOT binary
menus/v3/templates/* binary
```

## Status
- ⚠️ **Not blocking PR merge** - System remains functional
- 📝 **Documented** for future maintenance
- 🔧 **Requires manual intervention** - Cannot be automated


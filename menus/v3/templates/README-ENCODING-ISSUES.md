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
- Recreate templates using ANSI editor with CP437 encoding
- Or restore from backup if available
- Ensure git handles these as binary files: .gitattributes entry needed


# ViSiON/3 File Areas Guide

This guide covers the file area system in ViSiON/3, including configuration, management, and usage.

## File System Overview

The file system allows users to browse, upload, and download files organized into topic-based areas. Each area can have its own access controls and settings.

## File Area Configuration

File areas are defined in `configs/file_areas.json`:

```json
[
  {
    "id": 1,
    "tag": "GENERAL",
    "name": "General Files",
    "description": "General purpose file area",
    "path": "general",
    "acs_list": "",
    "acs_upload": "",
    "acs_download": ""
  }
]
```

### Area Properties

- `id` - Unique numeric identifier
- `tag` - Short tag name (uppercase)
- `name` - Display name
- `description` - Area description
- `path` - Directory path relative to `data/files/`
- `acs_list` - ACS required to list files
- `acs_upload` - ACS required to upload
- `acs_download` - ACS required to download

## File Storage

Files are stored in the directory specified by the area's `path`:
- Base directory: `data/files/`
- Area directory: `data/files/{area-path}/`
- Each file retains its original name
- Metadata stored in `metadata.json` within each area directory

### File Metadata Structure

Each area's `metadata.json` contains:
```json
[
  {
    "id": "11111111-1111-1111-1111-111111111111",
    "area_id": 1,
    "filename": "TEST1.TXT",
    "description": "A simple test text file.",
    "size": 21,
    "uploaded_at": "2024-01-01T10:00:00Z",
    "uploaded_by": "System",
    "download_count": 1
  }
]
```

### Metadata Fields

- `id` - UUID for the file record
- `area_id` - Links to FileArea.ID
- `filename` - Actual filename on disk
- `description` - User-provided description
- `size` - File size in bytes
- `uploaded_at` - Upload timestamp
- `uploaded_by` - User handle who uploaded
- `download_count` - Number of downloads

## File Functions

### Listing File Areas

The `LISTFILEAR` function displays available areas:
- Shows areas user has read access to
- Displays area ID, tag, name, and file count
- Uses templates: `FILEAREA.TOP`, `FILEAREA.MID`, `FILEAREA.BOT`

### Selecting File Area

The `SELECTFILEAREA` function allows area selection:
- Displays area list first
- Prompts for area number or tag
- Updates user's current file area
- Accepts both ID numbers and tag names

### Listing Files

The `LISTFILES` function shows files in current area:
- Displays filename, size, date, uploader, description
- Shows marking status with `*` for tagged files
- Paginated display (15 files per page)
- Commands: N=Next, P=Previous, #=Mark/Unmark, D=Download, Q=Quit

### File Operations

**Implemented:**
- **Browse**: Navigate paginated file listings
- **Mark/Unmark**: Tag files for batch download using file numbers
- **Download**: ZMODEM transfer using `sz` command

**In Development:**
- **Upload**: Send files to BBS
- **View**: Read text files or view archive contents

## Access Control

### Viewing Files
- Empty `acs_list` allows public viewing
- Restrict with: `s10`, `fD`, etc.

### Uploading Files
- Controlled by `acs_upload` setting
- Typically requires validation: `s10`

### Downloading Files
- Controlled by `acs_download` setting
- All files in an area share the same download permissions

## Creating a New File Area

1. Create directory structure:
```bash
mkdir -p data/files/myarea
```

2. Add to `configs/file_areas.json`:
```json
{
  "id": 2,
  "tag": "MYAREA",
  "name": "My File Area",
  "description": "Description here",
  "path": "myarea",
  "acs_list": "",
  "acs_upload": "s10",
  "acs_download": ""
}
```

3. Create empty metadata file:
```bash
echo '[]' > data/files/myarea/metadata.json
```

4. Restart BBS or reload configuration

## File Display Templates

File listings use templates in `menus/v3/templates/`:

### Area List Templates

**FILEAREA.TOP** - Header before area list
```
|07--- File Area List ---
```

**FILEAREA.MID** - Template for each area
```
 |07[^ID] |15^TAG - ^NA |07(^NF files)
```

**FILEAREA.BOT** - Footer after area list
```
|07--- End of List ---
```

### File List Templates

**FILELIST.TOP** - Header before file list
```
|07--- File List Top ---
```

**FILELIST.MID** - Template for each file
```
|15^MARK|07^NUM |11^NAME |07^DATE ^SIZE ^DESC
```

**FILELIST.BOT** - Footer with pagination
```
|07Page ^PAGE of ^TOTALPAGES
```

### Template Variables

**Area Templates:**
- `^ID` - Area ID number
- `^TAG` - Area tag
- `^NA` - Area name
- `^DS` - Area description
- `^NF` - Number of files

**File Templates:**
- `^MARK` - Selection marker (*) or space
- `^NUM` - File number on page
- `^NAME` - Filename
- `^SIZE` - File size (shown as XXXk)
- `^DATE` - Upload date (MM/DD/YY format)
- `^DESC` - File description

## File Management

### Adding Files Manually

1. Copy file to area directory:
```bash
cp myfile.zip data/files/general/
```

2. Update area's `metadata.json`:
```json
{
  "id": "44444444-4444-4444-4444-444444444444",
  "area_id": 1,
  "filename": "myfile.zip",
  "description": "My file",
  "size": 1024,
  "uploaded_at": "2025-01-01T00:00:00Z",
  "uploaded_by": "SysOp",
  "download_count": 0
}
```

Note: Generate a unique UUID for the `id` field.

### Removing Files

1. Delete the file:
```bash
rm data/files/general/oldfile.zip
```

2. Remove entry from `metadata.json`

### Batch Import

For importing many files:
1. Copy files to area directory
2. Create script to generate metadata entries with UUIDs
3. Append to existing `metadata.json`

## Best Practices

1. **Organization**: Group related files in appropriate areas
2. **Descriptions**: Provide clear, meaningful file descriptions
3. **Access Control**: Set appropriate ACS levels for each area
4. **File Integrity**: Verify files exist that are listed in metadata
5. **Backups**: Backup both files and metadata regularly

## File Transfer Protocols

**Currently Implemented:**
- **Zmodem**: Using external `sz` command for downloads

**Planned:**
- **Xmodem**: Fallback option
- **Ymodem**: Batch transfers
- **Upload support**: Using `rz` or similar

## Troubleshooting

### Files Not Showing
- Check user's access level vs `acs_list`
- Verify `metadata.json` exists and is valid JSON
- Ensure files referenced in metadata exist on disk

### Can't Change Areas
- Verify area exists in `configs/file_areas.json`
- Check ACS requirements
- Area accepts both ID numbers and tags (case-insensitive)

### Download Issues
- Ensure `sz` command is installed and in PATH
- Check file permissions in area directory
- Verify terminal supports ZMODEM protocol
- User must have files marked before download

### Metadata Issues
- File IDs must be valid UUIDs
- `area_id` must match the area configuration
- JSON syntax must be valid (use array format)

## Future Enhancements

Planned file system improvements:
- Full upload implementation
- View text files and ZIP contents inline
- File searching across areas
- Archive viewing without download
- File approval queue for uploads
- Duplicate checking
- Virus scanning integration
- File request system

## File Area Maintenance

### Regular Tasks
- Verify metadata matches actual files
- Remove orphaned metadata entries
- Update descriptions for clarity
- Check for duplicate files
- Monitor disk usage

### Storage Considerations
- Plan for growth in popular areas
- Set upload size limits
- Implement user quotas
- Archive old/unused files

### Security
- Restrict executable uploads via ACS
- Scan uploads for malware (when implemented)
- Monitor for inappropriate content
- Regular permission audits 
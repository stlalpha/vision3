# ViSiON/3 String Configuration Manager

A Turbo Pascal-style TUI application for editing BBS string configurations with real-time ANSI color preview.

## Features

### Multi-Pane Interface
- **Categories Pane**: Browse string categories (Login, Messages, Files, etc.)
- **String List Pane**: View all strings in the selected category
- **Editor Pane**: Edit strings with ANSI color support
- **Preview Pane**: Real-time formatted output preview

### ANSI Color Support
- Full support for ViSiON/2 color codes (`|00`-`|15`, `|B0`-`|B7`, `|C1`-`|C7`)
- Interactive color picker with vintage-style dialog boxes
- Live preview showing actual ANSI colors
- Special codes support (`|CL`, `|P`, `|PP`, `|23`)

### Advanced Editing Features
- Search and filter functionality across all strings
- Undo/redo support with full history tracking
- Bulk import/export of configurations
- String validation and template checking
- Category-based organization of 95+ configurable strings

### Turbo Pascal Aesthetic
- Classic blue background with yellow highlights
- IBM PC box drawing characters
- Vintage dialog boxes and menus
- Keyboard-driven navigation
- Classic status bar with function key hints

## Usage

### Basic Navigation
- `Tab` / `Shift+Tab`: Switch between panes
- `Enter`: Select item or edit string
- `Esc`: Cancel or go back
- `q` / `Ctrl+C`: Quit application

### String Editing
- `F2`: Open color picker dialog
- `F3`: Toggle preview pane
- `Ctrl+S`: Save current string
- `Ctrl+Z` / `Ctrl+Y`: Undo/Redo changes

### Search and Tools
- `/`: Search strings
- `F4`: Export configuration
- `F5`: Import configuration
- `r`: Toggle raw view (in preview pane)
- `?`: Show help

### Color Picker
- `↑/↓`: Navigate colors
- `←/→`: Navigate categories
- `Enter`: Insert selected color code
- `Esc`: Cancel color picker

## Architecture

### Core Components

#### StringManager (`manager.go`)
- Loads and manages the `strings.json` configuration
- Provides categorization and field mapping
- Handles undo/redo history
- Manages search and filtering
- Exports/imports JSON data

#### ANSI Helper (`ansi.go`)
- Processes ViSiON/2 color codes
- Provides color information and palettes
- Handles ANSI preview rendering
- Color code validation and suggestions

#### TUI Components (`components.go`)
- CategoryPane: Category browser
- StringListPane: String list with search
- EditorPane: Text editor with color support
- PreviewPane: Live ANSI preview

#### Styling System (`styles.go`)
- Turbo Pascal color theme
- Consistent styling across components
- Box drawing characters
- Window and dialog styling

#### Dialog System (`dialogs.go`)
- Modal dialogs for various operations
- File picker for import/export
- Color picker interface
- Error and confirmation dialogs

### Integration Points

#### With Existing Codebase
- Uses existing `config.StringsConfig` structure
- Integrates with `internal/ansi` package for color processing
- Compatible with existing configuration loading patterns
- Leverages Charm TUI framework (Bubble Tea, Bubbles, Lipgloss)

#### Configuration Files
- Reads from `configs/strings.json`
- Maintains compatibility with existing format
- Supports backup and restore operations

## String Categories

The manager organizes strings into logical categories:

- **Login**: Authentication and user login prompts
- **Messages**: Message system and board prompts
- **Files**: File transfer and area management
- **User**: User management and profile settings
- **System**: System messages and status displays
- **Mail**: Email and feedback system
- **Chat**: Chat and communication features
- **Prompts**: General UI prompts and interactions
- **Colors**: Default color code settings

## ANSI Color Codes

### Standard Colors (`|00` - `|15`)
- `|00` - `|07`: Standard colors (Black, Red, Green, Brown, Blue, Magenta, Cyan, Gray)
- `|08` - `|15`: Bright colors (Dark Gray, Bright Red, etc.)

### Background Colors (`|B0` - `|B7`)
- `|B0` - `|B7`: Background colors matching standard palette

### Custom Colors (`|C1` - `|C7`)
- Configurable colors mapped to system defaults

### Special Codes
- `|CL`: Clear screen and home cursor
- `|P`: Save cursor position
- `|PP`: Restore cursor position
- `|23`: Reset all attributes

## Development

### Building
```bash
cd cmd/stringtool
go build -o stringtool
```

### Running
```bash
./stringtool -config /path/to/configs
```

### Dependencies
- Charm TUI framework (Bubble Tea, Bubbles, Lipgloss)
- Go 1.24+ for type inference and other modern features

### File Structure
```
internal/configtool/strings/
├── manager.go      # Core string management
├── ansi.go         # ANSI color handling
├── components.go   # TUI pane components
├── styles.go       # Turbo Pascal styling
├── dialogs.go      # Dialog system
├── tui.go          # Main TUI application
└── README.md       # This file

cmd/stringtool/
└── main.go         # Command-line interface
```

## Future Enhancements

- Template system for common string patterns
- Macro recording and playback
- Advanced search with regex support
- Theme customization options
- Integration with version control
- Multi-language support
- Plugin system for custom validators
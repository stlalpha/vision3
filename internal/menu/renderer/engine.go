package renderer

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Config is the runtime renderer configuration built from persisted settings.
type Config struct {
	Enabled           bool
	DefaultTheme      string
	Palette           string
	Codepage          string
	AllowExternalAnsi bool
	MenuOverrides     map[string]Override
}

// Override customises rendering for a specific menu.
type Override struct {
	Mode     string
	Theme    string
	Palette  string
	Codepage string
}

// MenuContext provides the dynamic information required to render a menu.
type MenuContext struct {
	Name  string
	User  UserInfo
	Stats Stats
}

// UserInfo holds information about the active user/session for rendering.
type UserInfo struct {
	Handle string
	Node   int
}

// Stats are lightweight computed values surfaced inside the menu.
type Stats struct {
	UnreadMessages int
	NewFiles       int
	ActiveDoors    int
	OnlineCount    int
	Ratio          string
}

// Engine renders programmatic menus based on the supplied configuration.
type Engine struct {
	cfg       Config
	palettes  map[string]Palette
	glyphMaps map[string]glyphMapper
}

// NewEngine initialises a renderer.Engine. If cfg.Enabled is false a nil engine is returned.
func NewEngine(cfg Config) *Engine {
	if !cfg.Enabled {
		return nil
	}

	engine := &Engine{
		cfg:       cfg,
		palettes:  buildPalettes(),
		glyphMaps: buildGlyphMappers(),
	}

	return engine
}

// Render constructs the ANSI output for the requested menu.
// The returned bool indicates whether the renderer handled the menu or if the caller should
// fall back to the legacy external ANSI pipeline.
func (e *Engine) Render(ctx MenuContext) ([]byte, bool, error) {
	if e == nil {
		return nil, false, nil
	}

	plan := e.resolvePlan(ctx.Name)
	if plan.mode == renderModeExternal {
		return nil, false, nil
	}

	palette, ok := e.palettes[plan.palette]
	if !ok {
		palette = e.palettes[defaultPalette]
	}

	mapper, ok := e.glyphMaps[plan.codepage]
	if !ok {
		mapper = e.glyphMaps[codepageUTF8]
	}

	var raw string
	switch plan.theme {
	case "visionx":
		raw = renderVisionX(ctx, palette)
	default:
		return nil, false, fmt.Errorf("unknown renderer theme: %s", plan.theme)
	}

	if !strings.HasSuffix(raw, "\n") {
		raw += "\n"
	}

	rendered := mapper.MapString(raw)
	return rendered, true, nil
}

// resolvePlan determines the effective theme/palette/codepage for the menu.
func (e *Engine) resolvePlan(menuName string) renderPlan {
	plan := renderPlan{
		mode:     renderModeBuiltIn,
		theme:    e.cfg.DefaultTheme,
		palette:  e.cfg.Palette,
		codepage: e.cfg.Codepage,
	}

	if e.cfg.MenuOverrides == nil {
		return plan
	}

	override, ok := e.cfg.MenuOverrides[strings.ToUpper(menuName)]
	if !ok {
		return plan
	}

	if strings.TrimSpace(override.Mode) != "" {
		switch strings.ToLower(override.Mode) {
		case "external":
			plan.mode = renderModeExternal
		case "built_in":
			plan.mode = renderModeBuiltIn
		}
	}

	if strings.TrimSpace(override.Theme) != "" {
		plan.theme = strings.ToLower(override.Theme)
	}
	if strings.TrimSpace(override.Palette) != "" {
		plan.palette = strings.ToLower(override.Palette)
	}
	if strings.TrimSpace(override.Codepage) != "" {
		plan.codepage = strings.ToLower(override.Codepage)
	}

	return plan
}

// renderPlan is the resolved instruction set for a specific menu.
type renderPlan struct {
	mode     renderMode
	theme    string
	palette  string
	codepage string
}

type renderMode int

const (
	renderModeBuiltIn renderMode = iota
	renderModeExternal
)

const (
	defaultPalette = "amiga"
	codepageUTF8   = "utf8"
)

// buildPalettes registers the built-in colour palettes.
func buildPalettes() map[string]Palette {
	return map[string]Palette{
		"amiga": {
			FrameCorner:     231,
			FrameHigh:       219,
			FrameLow:        139,
			FrameFade:       246,
			AccentPrimary:   87,
			AccentHighlight: 213,
			AccentSecondary: 123,
			TextPrimary:     251,
			TextSecondary:   239,
		},
		"ibm_pc": {
			FrameCorner:     231,
			FrameHigh:       201,
			FrameLow:        129,
			FrameFade:       250,
			AccentPrimary:   116,
			AccentHighlight: 213,
			AccentSecondary: 51,
			TextPrimary:     252,
			TextSecondary:   242,
		},
		"c64": {
			FrameCorner:     231,
			FrameHigh:       81,
			FrameLow:        19,
			FrameFade:       250,
			AccentPrimary:   123,
			AccentHighlight: 219,
			AccentSecondary: 81,
			TextPrimary:     253,
			TextSecondary:   243,
		},
	}
}

// Palette exposes helper methods to emit colour escape sequences.
type Palette struct {
	FrameCorner     int
	FrameHigh       int
	FrameLow        int
	FrameFade       int
	AccentPrimary   int
	AccentHighlight int
	AccentSecondary int
	TextPrimary     int
	TextSecondary   int
}

func (p Palette) Colour(code int) string {
	return fmt.Sprintf("\x1b[38;5;%dm", code)
}

func (p Palette) Reset() string {
	return "\x1b[0m"
}

// glyphMapper provides codepage specific replacements to keep visuals consistent.
type glyphMapper struct {
	replacements map[rune]rune
}

func buildGlyphMappers() map[string]glyphMapper {
	utf8Map := glyphMapper{replacements: map[rune]rune{}}

	amigaMap := glyphMapper{replacements: map[rune]rune{
		'⟢': '>',
		'┊': ':',
	}}

	c64Map := glyphMapper{replacements: map[rune]rune{
		'⟢': '>',
		'┊': ':',
	}}

	return map[string]glyphMapper{
		codepageUTF8:  utf8Map,
		"amiga_topaz": amigaMap,
		"c64_petscii": c64Map,
		"ibm_pc":      glyphMapper{replacements: map[rune]rune{}},
	}
}

func (gm glyphMapper) MapString(input string) []byte {
	if len(gm.replacements) == 0 {
		return []byte(input)
	}

	var builder strings.Builder
	builder.Grow(len(input))

	for _, r := range input {
		if replacement, ok := gm.replacements[r]; ok {
			builder.WriteRune(replacement)
		} else {
			builder.WriteRune(r)
		}
	}

	return []byte(builder.String())
}

// padVisible pads content to the desired visible width, ignoring ANSI sequences.
func padVisible(content string, width int) string {
	visible := visibleLength(content)
	if visible >= width {
		return content
	}
	padding := strings.Repeat(" ", width-visible)
	return content + padding
}

// visibleLength returns the printable rune length, ignoring ANSI control sequences.
func visibleLength(s string) int {
	length := 0
	inEscape := false

	for i := 0; i < len(s); {
		ch := s[i]
		if inEscape {
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				inEscape = false
			}
			i++
			continue
		}
		if ch == 0x1b { // ESC
			inEscape = true
			i++
			continue
		}
		if ch < 0x20 {
			i++
			continue
		}

		_, size := utf8.DecodeRuneInString(s[i:])
		length++
		i += size
	}
	return length
}

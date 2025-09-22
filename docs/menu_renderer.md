# Menu Renderer Overview

ViSiON/3 now includes a programmatic menu renderer that delivers stylised ANSI output without relying on pre-rendered `.ANS` art. The renderer is enabled by default and can be tuned through `configs/menu_renderer.json`.

## Configuration (`configs/menu_renderer.json`)

```json
{
  "enable": true,
  "defaultTheme": "visionx",
  "palette": "amiga",
  "codepage": "amiga_topaz",
  "allowExternalAnsi": true,
  "menuOverrides": {
    "LOGIN": { "mode": "external" }
  }
}
```

- `enable`: Toggles the renderer on/off globally.
- `defaultTheme`: Theme to use when no per-menu override is present. `visionx` is the built-in /X-inspired theme.
- `palette`: Colour palette. Options include `amiga`, `ibm_pc`, and `c64`.
- `codepage`: Glyph mapping strategy. Choose from `utf8`, `amiga_topaz`, `ibm_pc`, or `c64_petscii`.
- `allowExternalAnsi`: Retain compatibility with legacy `.ANS` assets. Menus marked `external` in overrides always use the ANSI loader.
- `menuOverrides`: Per-menu adjustments. Set `mode` to `external` to force legacy behaviour or `built_in` to opt-in explicitly. `LOGIN` defaults to external because it depends on coordinate markers in the legacy art.

## Theme “visionx”

The default theme replicates the low/high intensity magenta + cyan aesthetic of /X on the Amiga:

- Gradient frames built from `+`, `-`, and `|` in tiered colours (white → high magenta → low magenta → grey).
- Programmatic header/footer flourishes (`.(0o).`) and menu bullets (`⟢`) with per-codepage fallbacks.
- Dynamic content: unread message count, file totals, door counts, online nodes, and user ratio.

## Dynamic Data

During menu execution the executor assembles a `renderer.MenuContext` containing:

- User handle and current node ID.
- Aggregated message and file counts (sums across accessible areas).
- Active door count and online node estimate (current session for now).
- A basic ratio derived from uploads/logons (clamped to `0–999%`).

The renderer injects these values into themed slots. Additional stats can be wired in through `MenuExecutor.buildMenuContext`.

## Codepage Handling

The renderer applies lightweight glyph substitution before control passes to the existing terminal pipeline. This keeps the art legible when switching between CP437, Amiga Topaz, and PETSCII terminals. Glyph replacements can be extended via `internal/menu/renderer/engine.go` (`buildGlyphMappers`).

## External ANSI Compatibility

If a menu is marked `external` or the renderer is disabled, the original `.ANS` files are loaded exactly as before. Programmatic menus still honour ACS checks, pause prompts, and lightbar workflows — the renderer only replaces the static art layer.

## Preview

For quick previews without launching the BBS, cat the sample at `demos/visionx_demo.ans`:

```bash
cat demos/visionx_demo.ans
```

This is the same layout produced when the renderer drives `MAIN` with the default configuration.

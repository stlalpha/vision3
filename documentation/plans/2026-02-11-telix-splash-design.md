# Telix Dialer Splash Screen — Design

## Goal

First-time visitors see a full-screen simulation of Telix v3.51 dialing a BBS over a 14.4k modem before the site loads. Cookie gates it to one play per 30 days. No skip button — commit to the bit.

## Architecture

Three changes to the existing static site:

1. **HTML** — `#telix-splash` overlay div in `index.html`, z-index 10000 (above all CRT overlays). Contains click-to-start prompt, terminal output area, Telix status bar at bottom.
2. **CSS** — Full-viewport fixed overlay, 4:3 aspect ratio lock (`max-width: calc(100vh * 4 / 3)`), black letterboxing, CP437 font. Status bar styled like real Telix (cyan/white on dark blue). Blinking cursor.
3. **JS** — Cookie check, Web Audio API synthesis, audio file playback, typewriter text sequencer.
4. **Audio** — `docs/audio/modem-handshake.mp3` (~50-100KB) for the modem negotiation screech.

## Gating

- Check for cookie `vision3_visited=1` on page load
- If found: splash overlay is removed from DOM immediately, site loads normally
- If not found: overlay is visible, site content hidden behind it, scroll locked
- Cookie set with 30-day expiry after sequence completes

## Browser Autoplay Handling

Click-to-start screen shown first. User click creates AudioContext and unlocks audio playback. Sequence begins after click.

## 4:3 Aspect Ratio

Splash uses same approach as `.crt-frame`: `max-width: calc(100vh * 4 / 3)`, centered, black body background for letterboxing. CRT effects (scanlines, vignette, VHS tracking, phosphor flicker) render on top of the splash — same monitor.

## Sequence Timeline (~9 seconds)

| Time  | Terminal Output                          | Audio                                    |
|-------|------------------------------------------|------------------------------------------|
| 0.0s  | `AT&F`                                   | —                                        |
| 0.4s  | `OK`                                     | —                                        |
| 0.8s  | `ATDT ` then digits type one by one      | Dial tone at ATDT, DTMF per digit        |
| 2.5s  | Full number visible, cursor blinks       | Ring tone (US cadence)                   |
| 4.5s  | `RINGING`                                | Ring continues                           |
| 5.5s  | Ring stops                               | Modem screech (mp3) starts               |
| 6.0s  | `CONNECT 14400/ARQ/V32bis/LAPM`          | Screech continues                        |
| 7.0s  | Brief pause                              | Screech fades                            |
| 8.0s  | Overlay removed, site revealed           | Silence                                  |

## Audio Details

**Synthesized (Web Audio API):**
- Dial tone: 350Hz + 440Hz continuous
- DTMF tones: Standard ITU two-frequency pairs per digit, ~100ms each
- Ring tone: 440Hz + 480Hz, 2s on / 4s off US cadence

**Recorded:**
- `docs/audio/modem-handshake.mp3` — real modem negotiation screech

## Telix Status Bar

Bottom of the 4:3 frame. Styled like the real Telix v3.51 status line:

```
 Telix v3.51 │ COM1:14400 │ N-8-1 │ ANSI-BBS │ 314-567-3833
```

Cyan/white text on dark blue background. Visible throughout the entire sequence.

## Click-to-Start Screen

Before the sequence begins, the terminal area shows:

```
[blinking cursor]
```

Or a minimal prompt. The Telix status bar is already visible. User clicks anywhere to start.

## Phone Number

`314-567-3833` — the real BBS number from the 1990s.

## Transition

No fade, no dissolve. After CONNECT and brief pause, the overlay is removed from the DOM. The site is simply there — as if the BBS answered. The content was in the DOM the whole time, just covered.

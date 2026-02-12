# Telix Dialer Splash Screen — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a first-visit Telix dialer simulation splash screen with synthesized modem audio and a real modem handshake recording.

**Architecture:** Modify existing `docs/index.html`, `docs/style.css`, `docs/main.js`. Add `docs/audio/modem-handshake.mp3`. All changes are HTML/CSS/JS only. No build step.

**Tech Stack:** Static HTML/CSS/JS, Web Audio API for tone synthesis.

---

## Task 1: Obtain modem handshake audio

**Files:**
- Create: `docs/audio/modem-handshake.mp3`

**Step 1: Find and download a real modem handshake/negotiation sound**

Need a short (3-5 second) recording of the modem carrier negotiation screech — the iconic sound after the phone is answered and modems start handshaking. Look for public domain or freely licensed modem sound clips. Download to `docs/audio/modem-handshake.mp3`.

If a suitable free clip can't be found online, use `ffmpeg` to trim a longer recording to just the negotiation portion (~3-5 seconds starting from the carrier detect screech).

**Step 2: Optimize file size**

```bash
ffmpeg -i docs/audio/modem-handshake.mp3 -b:a 64k -ac 1 -ar 22050 docs/audio/modem-optimized.mp3
mv docs/audio/modem-optimized.mp3 docs/audio/modem-handshake.mp3
```

Target: under 100KB. Mono, low bitrate is fine — it's supposed to sound like a phone line.

**Step 3: Commit**

```bash
git add docs/audio/
git commit -m "Add modem handshake audio for Telix splash"
```

---

## Task 2: Add splash screen HTML

**Files:**
- Modify: `docs/index.html`

**Step 1: Add the telix-splash overlay**

Add immediately after `<body>`, before `.crt-frame`:

```html
<!-- Telix dialer splash — first visit only -->
<div id="telix-splash" class="telix-splash">
    <div class="telix-screen">
        <div class="telix-terminal" id="telix-terminal">
            <span class="telix-cursor">&#9608;</span>
        </div>
        <div class="telix-status-bar">
            <span>Telix v3.51</span>
            <span>COM1:14400</span>
            <span>N-8-1</span>
            <span>ANSI-BBS</span>
            <span>314-567-3833</span>
        </div>
    </div>
</div>
```

**Step 2: Commit**

```bash
git add docs/index.html
git commit -m "Add Telix splash screen HTML overlay"
```

---

## Task 3: Add splash screen CSS

**Files:**
- Modify: `docs/style.css`

**Step 1: Add Telix splash styles**

```css
/* ---- Telix Splash Screen ---- */
.telix-splash {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: #000000;
    z-index: 10000;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
}

.telix-screen {
    max-width: calc(100vh * 4 / 3);
    width: 100%;
    height: 100%;
    background: #000000;
    display: flex;
    flex-direction: column;
    position: relative;
}

.telix-terminal {
    flex: 1;
    padding: 1.5rem;
    font-family: var(--font-dos);
    font-size: 1rem;
    line-height: 1.6;
    color: #aaaaaa;
    white-space: pre-wrap;
    overflow: hidden;
}

.telix-cursor {
    color: #aaaaaa;
    animation: telix-blink 0.8s step-end infinite;
}

@keyframes telix-blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0; }
}

.telix-status-bar {
    font-family: var(--font-dos);
    font-size: 0.85rem;
    background: #0000aa;
    color: #55ffff;
    padding: 0.3rem 1rem;
    display: flex;
    justify-content: space-between;
    flex-shrink: 0;
}

.telix-splash.hidden {
    display: none;
}

/* Lock scroll while splash is visible */
body.splash-active {
    overflow: hidden;
}
```

**Step 2: Add to reduced motion — skip animation, show splash briefly then dismiss**

Inside the existing `@media (prefers-reduced-motion: reduce)` block, add:

```css
.telix-cursor { animation: none; }
```

**Step 3: Commit**

```bash
git add docs/style.css
git commit -m "Add Telix splash screen styles with 4:3 lock"
```

---

## Task 4: Implement the dialer sequence in JavaScript

**Files:**
- Modify: `docs/main.js`

**Step 1: Add cookie helper functions**

```javascript
function hasVisitedBefore() {
    return document.cookie.includes('vision3_visited=1');
}

function setVisitedCookie() {
    var expires = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toUTCString();
    document.cookie = 'vision3_visited=1; expires=' + expires + '; path=/; SameSite=Lax';
}
```

**Step 2: Add Web Audio API tone synthesizer**

DTMF frequency pairs (ITU-T Q.23):
```
1: 697+1209  2: 697+1336  3: 697+1477
4: 770+1209  5: 770+1336  6: 770+1477
7: 852+1209  8: 852+1336  9: 852+1477
*: 941+1209  0: 941+1336  #: 941+1477
```

```javascript
var DTMF_FREQUENCIES = {
    '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
    '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
    '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
    '*': [941, 1209], '0': [941, 1336], '#': [941, 1477]
};

function playTone(audioContext, frequencies, duration, startTime) {
    frequencies.forEach(function (freq) {
        var oscillator = audioContext.createOscillator();
        var gainNode = audioContext.createGain();
        oscillator.type = 'sine';
        oscillator.frequency.value = freq;
        gainNode.gain.value = 0.15;
        oscillator.connect(gainNode);
        gainNode.connect(audioContext.destination);
        oscillator.start(startTime);
        oscillator.stop(startTime + duration);
    });
}

function playDialTone(audioContext, startTime, duration) {
    playTone(audioContext, [350, 440], duration, startTime);
}

function playRingTone(audioContext, startTime) {
    // US ring: 440+480 Hz, 2s on, 4s off
    playTone(audioContext, [440, 480], 2.0, startTime);
}
```

**Step 3: Add typewriter text output function**

```javascript
function typeText(terminal, text, callback) {
    var index = 0;
    var cursor = terminal.querySelector('.telix-cursor');
    function typeChar() {
        if (index < text.length) {
            cursor.insertAdjacentText('beforebegin', text[index]);
            index++;
            setTimeout(typeChar, 30 + Math.random() * 20);
        } else if (callback) {
            callback();
        }
    }
    typeChar();
}

function printLine(terminal, text) {
    var cursor = terminal.querySelector('.telix-cursor');
    cursor.insertAdjacentText('beforebegin', text + '\n');
}
```

**Step 4: Add the main dialer sequence**

```javascript
function runDialerSequence(splash) {
    var terminal = document.getElementById('telix-terminal');
    var audioContext = new (window.AudioContext || window.webkitAudioContext)();
    var phoneDigits = '3145673833';
    var sequenceStart = audioContext.currentTime;

    // Preload modem screech
    var modemAudio = new Audio('audio/modem-handshake.mp3');
    modemAudio.volume = 0.5;

    // Phase 1: AT&F → OK (0.0s - 0.8s)
    printLine(terminal, 'AT&F');
    setTimeout(function () {
        printLine(terminal, 'OK');
    }, 400);

    // Phase 2: ATDT + dial digits (0.8s - ~2.5s)
    setTimeout(function () {
        var cursor = terminal.querySelector('.telix-cursor');
        cursor.insertAdjacentText('beforebegin', 'ATDT ');

        // Brief dial tone before digits
        playDialTone(audioContext, audioContext.currentTime, 0.3);

        // Type each digit with DTMF tone
        var digitIndex = 0;
        var digitInterval = setInterval(function () {
            if (digitIndex < phoneDigits.length) {
                var digit = phoneDigits[digitIndex];
                cursor.insertAdjacentText('beforebegin', digit);

                // Play DTMF for this digit
                var freqs = DTMF_FREQUENCIES[digit];
                if (freqs) {
                    playTone(audioContext, freqs, 0.1, audioContext.currentTime);
                }
                digitIndex++;
            } else {
                clearInterval(digitInterval);
                cursor.insertAdjacentText('beforebegin', '\n');
            }
        }, 120);
    }, 800);

    // Phase 3: Ringing (2.8s - 5.5s)
    setTimeout(function () {
        printLine(terminal, '');
        // Play ring tone twice
        playRingTone(audioContext, audioContext.currentTime);
        setTimeout(function () {
            printLine(terminal, 'RINGING');
            playRingTone(audioContext, audioContext.currentTime);
        }, 1200);
    }, 2800);

    // Phase 4: Modem screech + CONNECT (5.5s - 7.0s)
    setTimeout(function () {
        modemAudio.play().catch(function () {});
        setTimeout(function () {
            printLine(terminal, '');
            printLine(terminal, 'CONNECT 14400/ARQ/V32bis/LAPM');
        }, 500);
    }, 5500);

    // Phase 5: Remove overlay (8.0s)
    setTimeout(function () {
        modemAudio.pause();
        audioContext.close();
        splash.remove();
        document.body.classList.remove('splash-active');
        setVisitedCookie();
    }, 8000);
}
```

**Step 5: Add initialization — tie it all together**

```javascript
(function () {
    var splash = document.getElementById('telix-splash');
    if (!splash) return;

    if (hasVisitedBefore()) {
        splash.remove();
        return;
    }

    document.body.classList.add('splash-active');

    splash.addEventListener('click', function handler() {
        splash.removeEventListener('click', handler);
        splash.style.cursor = 'default';
        runDialerSequence(splash);
    });
})();
```

**Step 6: Commit**

```bash
git add docs/main.js
git commit -m "Implement Telix dialer sequence with DTMF synthesis and modem audio"
```

---

## Task 5: Test and refine

**Step 1: Test first visit (no cookie)**

- Clear cookies for the site
- Load page — should see Telix splash with blinking cursor and status bar
- Click — sequence should play with audio
- After ~8s, site appears
- Verify cookie is set

**Step 2: Test return visit (cookie present)**

- Reload page — splash should not appear, site loads immediately

**Step 3: Test reduced motion**

- Enable `prefers-reduced-motion` in browser dev tools
- Clear cookie, reload — cursor should not blink

**Step 4: Verify CRT effects overlay the splash**

- Scanlines, vignette, VHS tracking, phosphor flicker should all be visible over the Telix screen

**Step 5: Tweak timing if needed**

Adjust delays in `runDialerSequence()` to feel right.

**Step 6: Final commit if tweaks needed**

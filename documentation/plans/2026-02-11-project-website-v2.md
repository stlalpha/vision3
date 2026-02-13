# ViSiON/3 Project Website v2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Overhaul the existing website with enhanced CRT effects, rewritten copy in the README's voice, a history section with ViSiON/2 screenshots, and removal of the Tech Stack section.

**Architecture:** Modify existing docs/index.html, docs/style.css, docs/main.js in the feature/project-website worktree. Download 3 ViSiON/2 screenshots into docs/images/. All changes are CSS/HTML/JS only.

**Tech Stack:** Same as v1 — static HTML/CSS/JS, no frameworks, no build step.

---

## Task 1: Download and optimize ViSiON/2 screenshots

**Files:**
- Create: `docs/images/welcome.png`
- Create: `docs/images/mainmenu.png`
- Create: `docs/images/credits.png`

**Step 1: Create images directory**

```bash
mkdir -p docs/images
```

**Step 2: Download the three screenshots from the vision-2-bbs repo**

```bash
curl -sL "https://github.com/stlalpha/vision-2-bbs/blob/main/IMAGES/welcome.png?raw=true" -o docs/images/welcome.png
curl -sL "https://github.com/stlalpha/vision-2-bbs/blob/main/IMAGES/mainmenu.png?raw=true" -o docs/images/mainmenu.png
curl -sL "https://github.com/stlalpha/vision-2-bbs/blob/main/IMAGES/credits.png?raw=true" -o docs/images/credits.png
```

**Step 3: Optimize images with ImageMagick if they're oversized**

```bash
for img in docs/images/*.png; do
    magick "$img" -resize '800x>' -strip "$img"
done
```

**Step 4: Commit**

```bash
git add docs/images/
git commit -m "Add ViSiON/2 screenshots for history section"
```

---

## Task 2: Enhanced CRT effects in CSS

**Files:**
- Modify: `docs/style.css`

**Step 1: Replace the existing `.scanlines` rule with enhanced version**

Replace the current `.scanlines` block with:

```css
/* Scanline overlay - visible static lines */
.scanlines {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    z-index: 9999;
    background: repeating-linear-gradient(
        0deg,
        rgba(0, 0, 0, 0.08) 0px,
        rgba(0, 0, 0, 0.08) 1px,
        transparent 1px,
        transparent 3px
    );
}

/* Animated CRT sweep line */
.scanlines::after {
    content: '';
    position: fixed;
    top: -100%;
    left: 0;
    width: 100%;
    height: 8px;
    background: linear-gradient(
        180deg,
        transparent,
        rgba(68, 102, 255, 0.06),
        transparent
    );
    animation: scanline-sweep 8s linear infinite;
    pointer-events: none;
}

@keyframes scanline-sweep {
    0% { top: -1%; }
    100% { top: 101%; }
}

/* Occasional horizontal jitter */
@keyframes crt-jitter {
    0%, 97% { transform: translateX(0); }
    97.5% { transform: translateX(-3px); }
    98% { transform: translateX(2px); }
    98.5% { transform: translateX(-1px); }
    99% { transform: translateX(0); }
    100% { transform: translateX(0); }
}

body {
    animation: crt-jitter 18s ease-in-out infinite;
}
```

Note: the jitter keyframes concentrate the glitch into a tiny fraction of the cycle (97-99% of an 18s loop = glitch happens roughly every 18 seconds, lasts ~360ms).

**Step 2: Update prefers-reduced-motion to disable all new effects**

Replace the existing reduced-motion media query with:

```css
@media (prefers-reduced-motion: reduce) {
    html { scroll-behavior: auto; }
    .fade-in {
        opacity: 1;
        transform: none;
        transition: none;
    }
    .scanlines { background: none; }
    .scanlines::after { animation: none; }
    body { animation: none; }
}
```

**Step 3: Commit**

```bash
git add docs/style.css
git commit -m "Enhance CRT effects: visible scanlines, sweep line, horizontal jitter"
```

---

## Task 3: Add History section styles and screenshot gallery

**Files:**
- Modify: `docs/style.css`

**Step 1: Add history section styles to style.css**

Add after the About section styles:

```css
/* ---- History ---- */
.history p {
    margin-bottom: 1rem;
    max-width: 700px;
}

.lineage {
    font-family: var(--font-mono);
    font-size: 1.1rem;
    color: var(--cyan-accent);
    text-align: center;
    margin: 2rem 0;
    letter-spacing: 0.5px;
}

.lineage strong {
    color: var(--blue-bright);
    text-shadow: 0 0 10px rgba(68, 102, 255, 0.4);
}

.screenshot-gallery {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1rem;
    margin: 2rem 0;
}

.screenshot-gallery img {
    width: 100%;
    height: auto;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.screenshot-gallery img:hover {
    border-color: var(--blue-bright);
    box-shadow: 0 0 15px rgba(68, 102, 255, 0.15);
}

.screenshot-caption {
    font-family: var(--font-mono);
    font-size: 0.8rem;
    color: var(--text-secondary);
    text-align: center;
    margin-top: 0.25rem;
}

.history-links {
    margin-top: 1.5rem;
    font-family: var(--font-mono);
    font-size: 0.9rem;
}

.history-links a {
    color: var(--cyan-accent);
    text-decoration: none;
    margin-right: 1.5rem;
}

.history-links a:hover {
    text-decoration: underline;
}
```

**Step 2: Commit**

```bash
git add docs/style.css
git commit -m "Add history section and screenshot gallery styles"
```

---

## Task 4: Rewrite index.html with new content and structure

**Files:**
- Modify: `docs/index.html`

**Step 1: Rewrite the full index.html**

The new HTML structure (full replacement of the body content):

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="description" content="ViSiON/3 BBS - A ground-up rewrite of ViSiON/2 in Go. SSH, not dialup. Everything else, the real deal.">
    <title>ViSiON/3 BBS</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;600;700&family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="scanlines"></div>

    <!-- Hero -->
    <header class="hero">
        <div class="container">
            <img src="ViSiON3.png" alt="ViSiON/3 BBS" class="hero-logo">
            <p class="tagline">A modern resurrection of the classic BBS experience</p>
            <p class="subtitle">Written in Go. Connected via SSH. No web apps here.</p>
        </div>
    </header>

    <!-- History -->
    <section class="history" id="history">
        <div class="container">
            <h2>The Lineage</h2>
            <p class="lineage">FORUM &rarr; LSD &rarr; ViSiON &rarr; ViSiON/2 &rarr; <strong>ViSiON/3</strong></p>
            <p>ViSiON/2 was written by Crimson Blade, circa 1991-1993. It was a forum hack — descended from FORUM through LSD through ViSiON. It was beautiful out of the box, insanely configurable, and could emulate any other BBS you could think of through its completely user-scriptable workflow. It was my absolute favorite of all the hundreds of BBS programs I played with, and the only one I ever worked on.</p>
            <p>I got to know Crimson Blade around '91 or '92. We were kids — working together on a software project, servicing users all over the country, in a mostly pre-internet world. We spent an enormous amount of time on the phone and on party lines, hanging out and working on V2. ViSiON/3 is built in tribute to that software and those friendships.</p>
            <div class="screenshot-gallery">
                <div>
                    <img src="images/welcome.png" alt="ViSiON/2 Welcome Screen">
                    <p class="screenshot-caption">Welcome screen</p>
                </div>
                <div>
                    <img src="images/mainmenu.png" alt="ViSiON/2 Main Menu">
                    <p class="screenshot-caption">Main menu</p>
                </div>
                <div>
                    <img src="images/credits.png" alt="ViSiON/2 Credits">
                    <p class="screenshot-caption">Credits</p>
                </div>
            </div>
            <div class="history-links">
                <a href="https://github.com/stlalpha/vision-2-bbs">See the original source &rarr;</a>
                <a href="https://www.youtube.com/watch?v=Dddbe9OuJLU&list=PL7nj3G6Jpv2G6Gp6NvN1kUtQuW8QshBWE">Watch the BBS Documentary &rarr;</a>
            </div>
        </div>
    </section>

    <!-- About -->
    <section class="about" id="about">
        <div class="container">
            <h2>About</h2>
            <p>This is a ground-up rewrite of ViSiON/2 BBS in Go. You connect over SSH, not dialup — but everything else is the real deal. Menus, message bases, file areas, door games, ANSI art. We didn't modernize away what makes it a BBS. We're not going to turn it into a web app. We're not adding a REST API. If you want that, there are plenty of other projects that would love to have you.</p>
            <p class="tribute">Built in tribute to ViSiON/2 and Crimson Blade.</p>
        </div>
    </section>

    <!-- Features -->
    <section class="features" id="features">
        <div class="container">
            <h2>What It Does</h2>
            <div class="feature-grid">
                <div class="feature-card">
                    <h3>SSH, Not Dialup</h3>
                    <p>Connect over SSH. Any terminal. Any client. Its not 1993 -- we don't need to tie up your mom's phone line, but this ain't point and click.</p>
                </div>
                <div class="feature-card">
                    <h3>Messages &amp; File Areas</h3>
                    <p>Compose, read and newscan messages. But lets be honest, most of you just browsed the file areas. This was the core of what made a BBS a BBS.</p>
                </div>
                <div class="feature-card">
                    <h3>Door Games</h3>
                    <p>Run external programs with dropfile generation. TradeWars, L.O.R.D, FoodFight? They were terrible but alot of people loved them - now you can too.</p>
                </div>
                <div class="feature-card">
                    <h3>ANSI Art</h3>
                    <p>Never have so few done so much with so little. And damn it was pretty! CP437 and UTF-8 support. If you don't know what TheDraw is, you don't care about this either.</p>
                </div>
                <div class="feature-card">
                    <h3>Menus &amp; ACS</h3>
                    <p>.MNU, .CFG, .ANS files — the whole setup. Fully scriptable, completely configurable. Your board, your rules, your access levels.</p>
                </div>
            </div>
        </div>
    </section>

    <!-- Get Started -->
    <section class="get-started" id="get-started">
        <div class="container">
            <h2>Get Started</h2>
            <div class="terminal-block">
                <div class="terminal-header" aria-hidden="true">
                    <span class="terminal-dot red"></span>
                    <span class="terminal-dot yellow"></span>
                    <span class="terminal-dot green"></span>
                </div>
                <pre><code>$ git clone https://github.com/stlalpha/vision3.git
$ cd vision3
$ ./setup.sh
$ cd cmd/vision3 && ./vision3

# Connect from another terminal
$ ssh felonius@localhost -p 2222</code></pre>
            </div>
        </div>
    </section>

    <!-- Get Involved -->
    <section class="get-involved" id="get-involved">
        <div class="container">
            <h2>Get Involved</h2>
            <p>Do you write Go? Do you have fond memories of waiting 3 minutes for a single GIF to download at 14.4k? Are you looking for a project that will impress exactly nobody at your day job but might make a dozen middle-aged nerds unreasonably happy? Submit PRs. Jump in the Discord. Your reward is the satisfaction of knowing that somewhere, someone is reliving their misspent youth thanks to your code.</p>
            <div class="cta-buttons">
                <a href="https://discord.gg/VkjRN2Ms" class="btn btn-primary">Join the Discord</a>
                <a href="https://github.com/stlalpha/vision3" class="btn btn-secondary">View on GitHub</a>
            </div>
            <p class="contact">Questions? <a href="mailto:spaceman@vision3bbs.com">spaceman@vision3bbs.com</a></p>
        </div>
    </section>

    <!-- Footer -->
    <footer class="site-footer">
        <div class="container">
            <p>ViSiON/3 &mdash; Built in tribute to ViSiON/2 and Crimson Blade</p>
            <div class="footer-links">
                <a href="https://github.com/stlalpha/vision3">GitHub</a>
                <a href="https://discord.gg/VkjRN2Ms">Discord</a>
                <a href="mailto:spaceman@vision3bbs.com">Contact</a>
            </div>
        </div>
    </footer>

    <script src="main.js"></script>
</body>
</html>
```

**Step 2: Commit**

```bash
git add docs/index.html
git commit -m "Rewrite site content: history section, voice overhaul, kill tech stack"
```

---

## Task 5: Remove tech stack CSS (cleanup)

**Files:**
- Modify: `docs/style.css`

**Step 1: Remove the tech stack CSS rules**

Remove the entire `/* ---- Tech Stack ---- */` section including `.tech-list`, `.tech-list li`, and `.tech-label` rules.

**Step 2: Commit**

```bash
git add docs/style.css
git commit -m "Remove unused tech stack styles"
```

---

## Task 6: Verify and test locally

**Step 1: Serve locally**

```bash
cd docs && python3 -m http.server 8000
```

**Step 2: Verify**

- Scanlines visible but not overpowering
- Sweep line animates smoothly
- Jitter fires roughly every 18s, subtle
- History section: lineage chain, story text, 3 screenshots render
- Screenshot gallery responsive on mobile
- All rewritten copy reads correctly
- No tech stack section present
- Feature cards show 5 items with new copy
- Discord/GitHub links work
- CRT effects disabled when prefers-reduced-motion is set

**Step 3: Final commit if tweaks needed**

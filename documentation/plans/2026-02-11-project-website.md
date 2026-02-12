# ViSiON/3 Project Website Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a static single-page website for vision3bbs.com, hosted on GitHub Pages from the `docs/` directory.

**Architecture:** Static HTML/CSS/JS site with no build step. Single `index.html` with inline structure, external `style.css` for retro BBS aesthetic, minimal `main.js` for subtle effects. Dark theme with blue palette from ViSiON/3 logo. Hosted via GitHub Pages from `docs/` on `main` branch with custom domain.

**Tech Stack:** HTML5, CSS3, vanilla JavaScript, GitHub Pages, Google Fonts (IBM Plex Mono)

---

## Task 1: Rename `docs/` to `documentation/`

**Files:**
- Rename: `docs/` → `documentation/`
- Modify: `README.md` (lines 138, 183)
- Modify: `tasks/tasks.md` (line 132)

**Step 1: Move the directory**

```bash
git mv docs documentation
```

**Step 2: Update README.md references**

Change line 138:
```
See `docs/status.md` for detailed progress
```
to:
```
See `documentation/status.md` for detailed progress
```

Change line 183 in the project structure tree:
```
├── docs/                # Documentation
```
to:
```
├── documentation/       # Project documentation
```

**Step 3: Update tasks/tasks.md reference**

Change line 132:
```
- [x] Update project documentation (`README.md`, `docs/`) to reflect current features and architecture.
```
to:
```
- [x] Update project documentation (`README.md`, `documentation/`) to reflect current features and architecture.
```

**Step 4: Verify no broken references**

Run: `grep -r "docs/" --include="*.md" --include="*.go" .`
Expected: No results referencing old `docs/` path (except `docs/plans/` which is the website)

**Step 5: Commit**

```bash
git add -A
git commit -m "Rename docs/ to documentation/ to free docs/ for GitHub Pages website"
```

---

## Task 2: Create `docs/` directory structure for website

**Files:**
- Create: `docs/index.html`
- Create: `docs/style.css`
- Create: `docs/main.js`
- Create: `docs/CNAME`
- Copy: `ViSiON3.png` → `docs/ViSiON3.png`

**Step 1: Create the docs directory and CNAME**

```bash
mkdir -p docs
```

Write `docs/CNAME`:
```
vision3bbs.com
```

**Step 2: Copy the logo**

```bash
cp ViSiON3.png docs/ViSiON3.png
```

**Step 3: Commit skeleton**

```bash
git add docs/CNAME docs/ViSiON3.png
git commit -m "Add website skeleton with CNAME and logo"
```

---

## Task 3: Create the HTML structure

**Files:**
- Create: `docs/index.html`

**Step 1: Write index.html**

Complete single-page HTML with semantic sections. All content derived from README.md.

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="description" content="ViSiON/3 - A modern resurrection of the classic BBS experience, written in Go.">
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

    <!-- About -->
    <section class="about" id="about">
        <div class="container">
            <h2>About</h2>
            <p>ViSiON/3 is a ground-up modernization of the classic ViSiON/2 BBS software, built in Go. The goal is to recreate the core BBS experience using modern technologies — SSH instead of dialup, JSON instead of flat files — while keeping everything that made BBSing great.</p>
            <p>This isn't a web app. This isn't a REST API with a React frontend. This is a BBS. You connect with an SSH client, navigate menus with keypresses, and read messages one thread at a time. The way it was meant to be.</p>
            <p class="tribute">Built in tribute to ViSiON/2 and my friend Crimson Blade.</p>
        </div>
    </section>

    <!-- Features -->
    <section class="features" id="features">
        <div class="container">
            <h2>Current Features</h2>
            <div class="feature-grid">
                <div class="feature-card">
                    <h3>SSH Server</h3>
                    <p>Full PTY support via gliderlabs/ssh. Connect with any SSH client.</p>
                </div>
                <div class="feature-card">
                    <h3>User System</h3>
                    <p>Authentication with bcrypt hashing. User persistence, stats, and profiles.</p>
                </div>
                <div class="feature-card">
                    <h3>Menu System</h3>
                    <p>Classic .MNU/.CFG/.ANS menu files. Access Control System with full operator support.</p>
                </div>
                <div class="feature-card">
                    <h3>Message Areas</h3>
                    <p>Compose, read, and newscan messages. Full message base with area selection.</p>
                </div>
                <div class="feature-card">
                    <h3>File Areas</h3>
                    <p>File listings, area selection, and browsing. Transfer protocols in development.</p>
                </div>
                <div class="feature-card">
                    <h3>Door Games</h3>
                    <p>External program support with dropfile generation. Run classic door games.</p>
                </div>
                <div class="feature-card">
                    <h3>Community Features</h3>
                    <p>One-liner wall, last callers display, user listing, and call history tracking.</p>
                </div>
                <div class="feature-card">
                    <h3>ANSI Art</h3>
                    <p>Full pipe code processing. CP437 and UTF-8 output modes. Period-correct rendering.</p>
                </div>
            </div>
        </div>
    </section>

    <!-- Tech Stack -->
    <section class="tech" id="tech">
        <div class="container">
            <h2>Tech Stack</h2>
            <ul class="tech-list">
                <li><span class="tech-label">Language</span> Go 1.24</li>
                <li><span class="tech-label">SSH</span> gliderlabs/ssh</li>
                <li><span class="tech-label">Terminal</span> golang.org/x/term</li>
                <li><span class="tech-label">Auth</span> golang.org/x/crypto/bcrypt</li>
                <li><span class="tech-label">Data</span> JSON</li>
            </ul>
        </div>
    </section>

    <!-- Get Started -->
    <section class="get-started" id="get-started">
        <div class="container">
            <h2>Get Started</h2>
            <div class="terminal-block">
                <div class="terminal-header">
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
            <p>ViSiON/3 is open source and we welcome contributors. Whether you write Go, create ANSI art, or just love the BBS aesthetic — there's a place for you.</p>
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
git commit -m "Add website HTML structure with all content sections"
```

---

## Task 4: Create the CSS stylesheet

**Files:**
- Create: `docs/style.css`

**Step 1: Write style.css**

Retro BBS aesthetic: dark background (#0a0a0a), blue palette from logo, monospace headings (IBM Plex Mono), clean body text (Inter), subtle scanline overlay, CRT glow effects on logo, terminal-style code blocks, responsive grid for feature cards.

```css
/* ============================================
   ViSiON/3 BBS - Project Website Styles
   Retro BBS aesthetic, modern layout
   ============================================ */

*, *::before, *::after {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

:root {
    --bg-primary: #0a0a0a;
    --bg-secondary: #111118;
    --bg-card: #15151f;
    --text-primary: #c8c8d4;
    --text-secondary: #8888a0;
    --text-heading: #e0e0f0;
    --blue-bright: #4466ff;
    --blue-glow: #3355dd;
    --blue-dark: #1a1a4e;
    --cyan-accent: #44ccdd;
    --border-color: #252540;
    --font-mono: 'IBM Plex Mono', 'Courier New', monospace;
    --font-body: 'Inter', -apple-system, sans-serif;
}

html {
    scroll-behavior: smooth;
}

body {
    background-color: var(--bg-primary);
    color: var(--text-primary);
    font-family: var(--font-body);
    font-size: 16px;
    line-height: 1.7;
    overflow-x: hidden;
}

/* Scanline overlay */
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
        rgba(0, 0, 0, 0.03) 0px,
        rgba(0, 0, 0, 0.03) 1px,
        transparent 1px,
        transparent 3px
    );
}

.container {
    max-width: 900px;
    margin: 0 auto;
    padding: 0 2rem;
}

/* ---- Hero ---- */
.hero {
    text-align: center;
    padding: 6rem 0 4rem;
}

.hero-logo {
    max-width: 320px;
    width: 100%;
    height: auto;
    filter: drop-shadow(0 0 30px rgba(68, 102, 255, 0.4))
            drop-shadow(0 0 60px rgba(68, 102, 255, 0.2));
    margin-bottom: 2rem;
}

.tagline {
    font-family: var(--font-mono);
    font-size: 1.4rem;
    font-weight: 600;
    color: var(--text-heading);
    margin-bottom: 0.5rem;
}

.subtitle {
    font-family: var(--font-mono);
    font-size: 0.95rem;
    color: var(--text-secondary);
}

/* ---- Sections ---- */
section {
    padding: 4rem 0;
}

section:nth-child(even) {
    background-color: var(--bg-secondary);
}

h2 {
    font-family: var(--font-mono);
    font-size: 1.6rem;
    font-weight: 700;
    color: var(--blue-bright);
    margin-bottom: 1.5rem;
    text-shadow: 0 0 20px rgba(68, 102, 255, 0.3);
}

/* ---- About ---- */
.about p {
    margin-bottom: 1rem;
    max-width: 700px;
}

.tribute {
    font-family: var(--font-mono);
    font-style: italic;
    color: var(--cyan-accent);
    margin-top: 1.5rem;
}

/* ---- Features ---- */
.feature-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 1.25rem;
    margin-top: 1.5rem;
}

.feature-card {
    background-color: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    padding: 1.5rem;
    transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.feature-card:hover {
    border-color: var(--blue-bright);
    box-shadow: 0 0 15px rgba(68, 102, 255, 0.15);
}

.feature-card h3 {
    font-family: var(--font-mono);
    font-size: 1rem;
    font-weight: 600;
    color: var(--cyan-accent);
    margin-bottom: 0.5rem;
}

.feature-card p {
    font-size: 0.9rem;
    color: var(--text-secondary);
    line-height: 1.5;
}

/* ---- Tech Stack ---- */
.tech-list {
    list-style: none;
    max-width: 500px;
}

.tech-list li {
    font-family: var(--font-mono);
    font-size: 0.95rem;
    padding: 0.6rem 0;
    border-bottom: 1px solid var(--border-color);
    display: flex;
    gap: 1rem;
}

.tech-label {
    color: var(--cyan-accent);
    font-weight: 600;
    min-width: 100px;
}

/* ---- Get Started / Terminal Block ---- */
.terminal-block {
    background-color: #0d0d0d;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    overflow: hidden;
    max-width: 650px;
    margin-top: 1.5rem;
}

.terminal-header {
    background-color: #1a1a2e;
    padding: 0.6rem 1rem;
    display: flex;
    gap: 0.4rem;
}

.terminal-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
}

.terminal-dot.red { background-color: #ff5f57; }
.terminal-dot.yellow { background-color: #ffbd2e; }
.terminal-dot.green { background-color: #28c840; }

.terminal-block pre {
    padding: 1.25rem;
    overflow-x: auto;
}

.terminal-block code {
    font-family: var(--font-mono);
    font-size: 0.85rem;
    color: #88cc88;
    line-height: 1.8;
}

/* ---- Get Involved ---- */
.get-involved {
    text-align: center;
}

.get-involved p {
    max-width: 600px;
    margin: 0 auto 1.5rem;
}

.cta-buttons {
    display: flex;
    gap: 1rem;
    justify-content: center;
    flex-wrap: wrap;
    margin-bottom: 2rem;
}

.btn {
    font-family: var(--font-mono);
    font-size: 0.95rem;
    font-weight: 600;
    text-decoration: none;
    padding: 0.75rem 2rem;
    border-radius: 4px;
    transition: all 0.2s ease;
}

.btn-primary {
    background-color: var(--blue-bright);
    color: #ffffff;
}

.btn-primary:hover {
    background-color: var(--blue-glow);
    box-shadow: 0 0 20px rgba(68, 102, 255, 0.4);
}

.btn-secondary {
    background-color: transparent;
    color: var(--blue-bright);
    border: 1px solid var(--blue-bright);
}

.btn-secondary:hover {
    background-color: var(--blue-dark);
    box-shadow: 0 0 20px rgba(68, 102, 255, 0.2);
}

.contact {
    font-size: 0.9rem;
    color: var(--text-secondary);
}

.contact a {
    color: var(--cyan-accent);
    text-decoration: none;
}

.contact a:hover {
    text-decoration: underline;
}

/* ---- Footer ---- */
.site-footer {
    text-align: center;
    padding: 3rem 0;
    border-top: 1px solid var(--border-color);
    font-size: 0.85rem;
    color: var(--text-secondary);
}

.footer-links {
    margin-top: 1rem;
    display: flex;
    gap: 1.5rem;
    justify-content: center;
}

.footer-links a {
    color: var(--blue-bright);
    text-decoration: none;
    font-family: var(--font-mono);
    font-size: 0.85rem;
}

.footer-links a:hover {
    text-decoration: underline;
}

/* ---- Responsive ---- */
@media (max-width: 600px) {
    .hero { padding: 4rem 0 3rem; }
    .hero-logo { max-width: 220px; }
    .tagline { font-size: 1.1rem; }
    section { padding: 3rem 0; }
    h2 { font-size: 1.3rem; }
    .feature-grid { grid-template-columns: 1fr; }
    .cta-buttons { flex-direction: column; align-items: center; }
}
```

**Step 2: Commit**

```bash
git add docs/style.css
git commit -m "Add retro BBS website stylesheet"
```

---

## Task 5: Add minimal JavaScript for subtle effects

**Files:**
- Create: `docs/main.js`

**Step 1: Write main.js**

Minimal JS: fade-in on scroll for sections, subtle logo pulse. No libraries.

```javascript
/* ViSiON/3 BBS - Minimal site effects */

(function () {
    'use strict';

    /* Fade-in sections on scroll */
    var sections = document.querySelectorAll('section, .hero');

    var observer = new IntersectionObserver(function (entries) {
        entries.forEach(function (entry) {
            if (entry.isIntersecting) {
                entry.target.classList.add('visible');
            }
        });
    }, { threshold: 0.1 });

    sections.forEach(function (section) {
        section.classList.add('fade-in');
        observer.observe(section);
    });
})();
```

**Step 2: Add corresponding CSS for fade-in to style.css**

Append to `docs/style.css`:

```css
/* ---- Scroll fade-in ---- */
.fade-in {
    opacity: 0;
    transform: translateY(20px);
    transition: opacity 0.6s ease, transform 0.6s ease;
}

.fade-in.visible {
    opacity: 1;
    transform: translateY(0);
}
```

**Step 3: Commit**

```bash
git add docs/main.js docs/style.css
git commit -m "Add scroll fade-in effects"
```

---

## Task 6: Update README project structure

**Files:**
- Modify: `README.md`

**Step 1: Update the project structure in README**

Add `docs/` as website directory and show `documentation/` as project docs in the tree.

**Step 2: Commit**

```bash
git add README.md
git commit -m "Update README project structure to reflect docs/ and documentation/ split"
```

---

## Task 7: Verify and test locally

**Step 1: Open in browser**

```bash
# Simple local server to test
cd docs && python3 -m http.server 8000
```

Open `http://localhost:8000` and verify:
- Logo renders with blue glow
- All sections present and readable
- Feature cards grid properly
- Terminal block looks correct
- Discord and GitHub buttons link correctly
- Mobile responsive (resize browser window)
- Scanline overlay is subtle, not distracting

**Step 2: Validate HTML**

Check no broken links, no missing images, clean structure.

**Step 3: Final commit if any tweaks needed**

---

## Task 8: Push and configure GitHub Pages

**Step 1: Push the branch**

```bash
git push -u origin feature/project-website
```

**Step 2: Create PR**

```bash
gh pr create --title "Add project website for vision3bbs.com" --body "..."
```

**Step 3: After merge — configure GitHub Pages**

In GitHub repo settings:
- Pages → Source: Deploy from branch
- Branch: `main`, folder: `/docs`
- Custom domain: `vision3bbs.com`

**Step 4: DNS configuration**

User must configure DNS for `vision3bbs.com`:
- CNAME record pointing to `stlalpha.github.io`
- Or A records pointing to GitHub Pages IPs

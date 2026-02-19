# ViSiON/3 Project Website v3 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Overhaul the website to feel like an actual BBS — message-base-styled content sections, V2/V3 menu-style navigation, ANSI art section headers, and menu-item-styled features.

**Architecture:** Modify existing docs/index.html, docs/style.css, docs/main.js. All changes are CSS/HTML/JS only. No build step.

**Tech Stack:** Static HTML/CSS/JS, no frameworks.

---

## Task 1: Add BBS nav bar

**Files:**
- Modify: `docs/index.html`
- Modify: `docs/style.css`

**Step 1: Add nav bar HTML**

Add after `<div class="scanlines"></div>` and before the hero:

```html
<nav class="bbs-nav">
    <div class="container">
        <span class="nav-item"><span class="nav-key">H</span>) <a href="#history">History</a></span>
        <span class="nav-item"><span class="nav-key">A</span>) <a href="#about">About</a></span>
        <span class="nav-item"><span class="nav-key">F</span>) <a href="#features">Features</a></span>
        <span class="nav-item"><span class="nav-key">G</span>) <a href="#get-started">Get Started</a></span>
        <span class="nav-item"><span class="nav-key">J</span>) <a href="#get-involved">Get Involved</a></span>
    </div>
</nav>
```

**Step 2: Add nav bar CSS**

```css
/* ---- BBS Nav Bar ---- */
.bbs-nav {
    position: sticky;
    top: 0;
    z-index: 100;
    background-color: var(--bg-primary);
    border-bottom: 1px solid var(--border-color);
    padding: 0.6rem 0;
    font-family: var(--font-mono);
    font-size: 0.9rem;
}

.bbs-nav .container {
    display: flex;
    justify-content: center;
    gap: 2rem;
    flex-wrap: wrap;
}

.nav-item {
    color: var(--text-secondary);
}

.nav-key {
    color: var(--cyan-accent);
    font-weight: 700;
}

.nav-item a {
    color: var(--text-primary);
    text-decoration: none;
}

.nav-item a:hover {
    color: var(--cyan-accent);
}

.nav-item a:focus-visible {
    outline: 2px solid var(--cyan-accent);
    outline-offset: 2px;
}
```

**Step 3: Commit**

```bash
git add docs/index.html docs/style.css
git commit -m "Add BBS-style navigation bar"
```

---

## Task 2: Create ANSI art section headers

**Files:**
- Modify: `docs/index.html`
- Modify: `docs/style.css`

**Step 1: Add ANSI art header CSS**

```css
/* ---- ANSI Art Headers ---- */
.ansi-header {
    font-family: var(--font-mono);
    line-height: 1.1;
    margin-bottom: 1.5rem;
    white-space: pre;
    font-size: 0.85rem;
}

.ansi-header .ansi-row-1 { color: #5577ff; }
.ansi-header .ansi-row-2 { color: #4466dd; }
.ansi-header .ansi-row-3 { color: #3355bb; }

@media (max-width: 600px) {
    .ansi-header {
        font-size: 0.55rem;
    }
}
```

**Step 2: Replace all h2 headings with ANSI art blocks**

Create block-letter art for each heading using ▄█▀░▒▓ characters. Three rows per header, each row a different shade of blue (bright → dark, top to bottom). The ANSI art replaces the `<h2>` elements.

Headers to create:
1. HISTORY
2. ABOUT
3. FEATURES
4. GET STARTED
5. GET INVOLVED

Each header is wrapped in:
```html
<div class="ansi-header" aria-label="History">
    <span class="ansi-row-1">...</span>
    <span class="ansi-row-2">...</span>
    <span class="ansi-row-3">...</span>
</div>
```

The aria-label provides accessibility since screen readers can't parse block art.

**Step 3: Commit**

```bash
git add docs/index.html docs/style.css
git commit -m "Add ANSI art section headers"
```

---

## Task 3: Style content sections as message-base posts

**Files:**
- Modify: `docs/index.html`
- Modify: `docs/style.css`

**Step 1: Add message post CSS**

```css
/* ---- Message Base Posts ---- */
.msg-header {
    font-family: var(--font-mono);
    font-size: 0.85rem;
    line-height: 1.6;
    margin-bottom: 0;
    padding: 0.75rem 1rem;
    background-color: var(--bg-card);
    border: 1px solid var(--border-color);
    border-bottom: none;
    border-radius: 4px 4px 0 0;
    max-width: 700px;
}

.msg-header .msg-label {
    color: var(--cyan-accent);
}

.msg-header .msg-value {
    color: var(--text-heading);
}

.msg-header .msg-right {
    float: right;
}

.msg-divider {
    font-family: var(--font-mono);
    color: var(--border-color);
    max-width: 700px;
    overflow: hidden;
    font-size: 0.85rem;
    padding: 0 1rem;
    background-color: var(--bg-card);
    border-left: 1px solid var(--border-color);
    border-right: 1px solid var(--border-color);
}

.msg-body {
    padding: 1rem;
    max-width: 700px;
    background-color: var(--bg-card);
    border: 1px solid var(--border-color);
    border-top: none;
    border-radius: 0 0 4px 4px;
}

.msg-body p {
    margin-bottom: 1rem;
    font-family: var(--font-body);
    line-height: 1.7;
}
```

**Step 2: Wrap History, About, Get Started, Get Involved sections in message post format**

Each section gets this structure (example for History, Msg# 1 of 5):

```html
<div class="msg-header">
    <span class="msg-label">Area:</span> <span class="msg-value">General</span>
    <span class="msg-right"><span class="msg-label">Msg#:</span> <span class="msg-value">1 of 5</span></span><br>
    <span class="msg-label">From:</span> <span class="msg-value">felonius</span>
    <span class="msg-right"><span class="msg-label">Date:</span> <span class="msg-value">02/11/26</span></span><br>
    <span class="msg-label">  To:</span> <span class="msg-value">All</span><br>
    <span class="msg-label">Subj:</span> <span class="msg-value">Where It Came From</span>
</div>
<div class="msg-divider">────────────────────────────────────────────────────────────────────</div>
<div class="msg-body">
    [existing section content goes here]
</div>
```

Message subjects:
- History: "Where It Came From"
- About: "What This Is"
- Get Started: "Running Your Own"
- Get Involved: "We Need You"

Features section does NOT get message post format — it uses menu-item styling (Task 4).

Note: Get Started is Msg# 3, Get Involved is Msg# 4. Features sits between them visually but isn't a message post so the numbering skips it: 1 (History), 2 (About), 3 (Get Started), 4 (Get Involved).

**Step 3: Commit**

```bash
git add docs/index.html docs/style.css
git commit -m "Style content sections as BBS message-base posts"
```

---

## Task 4: Restyle features as BBS menu items

**Files:**
- Modify: `docs/index.html`
- Modify: `docs/style.css`

**Step 1: Replace feature card HTML with menu-item layout**

```html
<section class="features" id="features">
    <div class="container">
        [ANSI art header for FEATURES]
        <div class="bbs-menu">
            <div class="bbs-menu-item">
                <span class="menu-key">S</span>) <span class="menu-title">SSH, Not Dialup</span>
                <p class="menu-desc">Connect over SSH. Any terminal. Any client. Its not 1993 -- we don't need to tie up your mom's phone line, but this ain't point and click.</p>
            </div>
            <div class="bbs-menu-item">
                <span class="menu-key">M</span>) <span class="menu-title">Messages &amp; File Areas</span>
                <p class="menu-desc">Compose, read and newscan messages. But lets be honest, most of you just browsed the file areas. This was the core of what made a BBS a BBS.</p>
            </div>
            <div class="bbs-menu-item">
                <span class="menu-key">D</span>) <span class="menu-title">Door Games</span>
                <p class="menu-desc">Run external programs with dropfile generation. TradeWars, L.O.R.D, FoodFight? They were terrible but alot of people loved them - now you can too.</p>
            </div>
            <div class="bbs-menu-item">
                <span class="menu-key">A</span>) <span class="menu-title">ANSI Art</span>
                <p class="menu-desc">Never have so few done so much with so little. And damn it was pretty! CP437 and UTF-8 support. If you don't know what TheDraw is, you don't care about this either.</p>
            </div>
            <div class="bbs-menu-item">
                <span class="menu-key">C</span>) <span class="menu-title">Menus &amp; ACS</span>
                <p class="menu-desc">.MNU, .CFG, .ANS files — the whole setup. Fully scriptable, completely configurable. Your board, your rules, your access levels.</p>
            </div>
            <div class="bbs-menu-item">
                <span class="menu-key">O</span>) <span class="menu-title">One-Liners &amp; Last Callers</span>
                <p class="menu-desc">The one-liner wall. Last callers list. Who's online. The stuff that made you feel like you weren't the only weirdo calling this thing at 2am.</p>
            </div>
        </div>
    </div>
</section>
```

**Step 2: Add menu item CSS**

```css
/* ---- BBS Menu Items (Features) ---- */
.bbs-menu {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1.5rem 3rem;
    font-family: var(--font-mono);
}

.bbs-menu-item {
    font-size: 0.9rem;
}

.menu-key {
    color: var(--cyan-accent);
    font-weight: 700;
}

.menu-title {
    color: var(--text-heading);
    font-weight: 600;
}

.menu-desc {
    color: var(--text-secondary);
    margin-top: 0.25rem;
    font-size: 0.85rem;
    line-height: 1.5;
    font-family: var(--font-body);
}

@media (max-width: 600px) {
    .bbs-menu {
        grid-template-columns: 1fr;
    }
}
```

**Step 3: Remove old feature card CSS**

Remove `.feature-grid`, `.feature-card`, `.feature-card:hover`, `.feature-card h3`, `.feature-card p` rules.

**Step 4: Commit**

```bash
git add docs/index.html docs/style.css
git commit -m "Restyle features as BBS menu items with two-column layout"
```

---

## Task 5: Clean up and verify

**Step 1: Remove any orphaned CSS rules**

Check for unused selectors (old `.about p` override if it conflicts with msg-body, old h2 styles since we're using ANSI headers now, etc.).

**Step 2: Verify locally**

```bash
cd docs && python3 -m http.server 8000
```

Check:
- Nav bar sticky and functional
- ANSI art headers render in blue gradient
- Message post format on History, About, Get Started, Get Involved
- Feature menu items in two columns on desktop, one on mobile
- Screenshots still render in History message body
- CRT effects still working
- All links correct
- Mobile responsive

**Step 3: Final commit if tweaks needed**

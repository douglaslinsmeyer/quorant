# Quorant Design System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete, interactive design system package with token files, live component examples, pattern documentation, and a Storybook-like component browser — all with light/dark mode support.

**Architecture:** Pure HTML/CSS with no build step. Token files are real CSS custom properties (importable by future apps) and a JSON mirror for React Native. Each component page is a self-contained HTML file that imports the token CSS. A component browser provides navigation and a live dark mode toggle.

**Tech Stack:** HTML, CSS (custom properties), vanilla JavaScript (dark mode toggle, component browser navigation), Node.js (token validation script)

**Spec:** `docs/superpowers/specs/2026-04-09-design-system-design.md`

---

## Task 1: Project scaffolding and .gitignore

**Files:**
- Create: `design-system/tokens/` (directory)
- Create: `design-system/components/` (directory)
- Create: `design-system/patterns/` (directory)
- Create: `.gitignore`

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p design-system/tokens design-system/components design-system/patterns
```

- [ ] **Step 2: Create .gitignore**

```gitignore
.superpowers/
node_modules/
.DS_Store
*.swp
```

- [ ] **Step 3: Commit**

```bash
git add .gitignore design-system/
git commit -m "chore: scaffold design-system directory structure"
```

---

## Task 2: Color tokens (light + dark)

**Files:**
- Create: `design-system/tokens/colors.css`

- [ ] **Step 1: Write colors.css with light mode tokens**

```css
/* design-system/tokens/colors.css
 * Quorant color system — light and dark mode via prefers-color-scheme
 * Import this file in any HTML page to get all color tokens.
 */

:root {
  /* === PRIMARY (Teal) === */
  --color-primary-900: #0A3D4F;
  --color-primary-800: #0E4F65;
  --color-primary-700: #13647E;
  --color-primary-600: #1A7A9A;
  --color-primary-500: #2391B4;
  --color-primary-400: #3DACC9;
  --color-primary-300: #6FC4DA;
  --color-primary-200: #A3DAE8;
  --color-primary-100: #D1EDF4;
  --color-primary-50:  #EBF7FB;

  /* === SECONDARY (Amber) === */
  --color-secondary-600: #B8860B;
  --color-secondary-500: #D4A017;
  --color-secondary-400: #E8BA3C;
  --color-secondary-200: #F5DFA0;
  --color-secondary-100: #FBF0D1;
  --color-secondary-50:  #FDF8EB;

  /* === NEUTRAL (Warm Gray) === */
  --color-neutral-950: #1A1A1A;
  --color-neutral-900: #2D2D2D;
  --color-neutral-700: #525252;
  --color-neutral-500: #7A7A7A;
  --color-neutral-400: #A3A3A3;
  --color-neutral-200: #E0E0E0;
  --color-neutral-100: #F0F0F0;
  --color-neutral-50:  #F8F8F8;
  --color-white:       #FFFFFF;

  /* === SEMANTIC === */
  --color-success-600: #16794A;
  --color-success-100: #D4EDDA;
  --color-error-600:   #C0392B;
  --color-error-100:   #F8D7DA;
  --color-warning-600: #D48806;
  --color-warning-100: #FFF3CD;
  --color-info-600:    #1A7A9A;
  --color-info-100:    #D1EDF4;

  /* === SURFACES === */
  --surface-page:     #F8F8F8;
  --surface-card:     #FFFFFF;
  --surface-elevated: #FFFFFF;
  --surface-sidebar:  linear-gradient(180deg, #072D3B, #0A3D4F);
  --surface-header:   linear-gradient(160deg, #072D3B, #0E4F65, #1A7A9A);

  /* === BACKDROP === */
  --backdrop: rgba(26, 26, 26, 0.5);
}

/* === DARK MODE === */
@media (prefers-color-scheme: dark) {
  :root {
    --color-primary-600: #3DACC9;
    --color-primary-500: #5BBDD6;
    --color-primary-400: #7ACEE3;
    --color-primary-300: #A3DAE8;
    --color-primary-200: #13647E;
    --color-primary-100: #0E4F65;
    --color-primary-50:  #0A3D4F;

    --color-secondary-500: #E8BA3C;

    --color-neutral-950: #F0F0F0;
    --color-neutral-900: #E0E0E0;
    --color-neutral-700: #A3A3A3;
    --color-neutral-500: #7A7A7A;
    --color-neutral-400: #525252;
    --color-neutral-200: #333333;
    --color-neutral-100: #262626;
    --color-neutral-50:  #1E1E1E;
    --color-white:       #2A2A2A;

    --color-success-600: #34C77A;
    --color-success-100: #1A3D2A;
    --color-error-600:   #E85C4A;
    --color-error-100:   #3D1A1A;
    --color-warning-600: #E8BA3C;
    --color-warning-100: #3D2E0A;
    --color-info-600:    #3DACC9;
    --color-info-100:    #0E4F65;

    --surface-page:     #141414;
    --surface-card:     #1E1E1E;
    --surface-elevated: #2A2A2A;
    --surface-sidebar:  linear-gradient(180deg, #0A0F14, #0F1A22);
    --surface-header:   linear-gradient(160deg, #0A0F14, #0F1A22, #13384A);

    --backdrop: rgba(0, 0, 0, 0.7);
  }
}

/* === MANUAL DARK MODE TOGGLE (class-based) === */
[data-theme="dark"] {
  --color-primary-600: #3DACC9;
  --color-primary-500: #5BBDD6;
  --color-primary-400: #7ACEE3;
  --color-primary-300: #A3DAE8;
  --color-primary-200: #13647E;
  --color-primary-100: #0E4F65;
  --color-primary-50:  #0A3D4F;

  --color-secondary-500: #E8BA3C;

  --color-neutral-950: #F0F0F0;
  --color-neutral-900: #E0E0E0;
  --color-neutral-700: #A3A3A3;
  --color-neutral-500: #7A7A7A;
  --color-neutral-400: #525252;
  --color-neutral-200: #333333;
  --color-neutral-100: #262626;
  --color-neutral-50:  #1E1E1E;
  --color-white:       #2A2A2A;

  --color-success-600: #34C77A;
  --color-success-100: #1A3D2A;
  --color-error-600:   #E85C4A;
  --color-error-100:   #3D1A1A;
  --color-warning-600: #E8BA3C;
  --color-warning-100: #3D2E0A;
  --color-info-600:    #3DACC9;
  --color-info-100:    #0E4F65;

  --surface-page:     #141414;
  --surface-card:     #1E1E1E;
  --surface-elevated: #2A2A2A;
  --surface-sidebar:  linear-gradient(180deg, #0A0F14, #0F1A22);
  --surface-header:   linear-gradient(160deg, #0A0F14, #0F1A22, #13384A);

  --backdrop: rgba(0, 0, 0, 0.7);
}
```

Note: The `[data-theme="dark"]` block duplicates the media query values. This allows the component browser to toggle dark mode via JavaScript (`document.documentElement.setAttribute('data-theme', 'dark')`) independent of OS preference.

- [ ] **Step 2: Verify the file is valid CSS**

Open `design-system/tokens/colors.css` in a browser dev tools console:

```bash
# Quick validation — parse and count custom properties
node -e "
const css = require('fs').readFileSync('design-system/tokens/colors.css', 'utf8');
const props = css.match(/--[\w-]+:/g);
console.log('Custom properties defined:', props.length);
console.log('Has dark media query:', css.includes('prefers-color-scheme: dark'));
console.log('Has data-theme toggle:', css.includes('data-theme'));
"
```

Expected: ~40+ custom properties, both dark mode mechanisms present.

- [ ] **Step 3: Commit**

```bash
git add design-system/tokens/colors.css
git commit -m "feat: add color tokens with light and dark mode"
```

---

## Task 3: Typography tokens

**Files:**
- Create: `design-system/tokens/typography.css`

- [ ] **Step 1: Write typography.css**

```css
/* design-system/tokens/typography.css
 * Quorant type system — Manrope headings, Inter body, JetBrains Mono code
 */

/* === FONT IMPORTS === */
@import url('https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700;800&family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap');

:root {
  /* === FONT FAMILIES === */
  --font-heading: 'Manrope', 'Segoe UI', system-ui, sans-serif;
  --font-body:    'Inter', 'Segoe UI', system-ui, sans-serif;
  --font-mono:    'JetBrains Mono', 'Cascadia Code', 'Consolas', monospace;

  /* === TYPE SCALE (1.25 ratio — Major Third) === */

  /* Display */
  --text-display-size: 3rem;           /* 48px */
  --text-display-weight: 800;
  --text-display-line-height: 1.1;
  --text-display-font: var(--font-heading);

  /* H1 */
  --text-h1-size: 2.25rem;            /* 36px */
  --text-h1-weight: 700;
  --text-h1-line-height: 1.2;
  --text-h1-font: var(--font-heading);

  /* H2 */
  --text-h2-size: 1.75rem;            /* 28px */
  --text-h2-weight: 700;
  --text-h2-line-height: 1.25;
  --text-h2-font: var(--font-heading);

  /* H3 */
  --text-h3-size: 1.375rem;           /* 22px */
  --text-h3-weight: 600;
  --text-h3-line-height: 1.3;
  --text-h3-font: var(--font-heading);

  /* H4 */
  --text-h4-size: 1.125rem;           /* 18px */
  --text-h4-weight: 600;
  --text-h4-line-height: 1.35;
  --text-h4-font: var(--font-heading);

  /* Body Large */
  --text-body-lg-size: 1.125rem;      /* 18px */
  --text-body-lg-weight: 400;
  --text-body-lg-line-height: 1.6;
  --text-body-lg-font: var(--font-body);

  /* Body */
  --text-body-size: 1rem;             /* 16px */
  --text-body-weight: 400;
  --text-body-line-height: 1.6;
  --text-body-font: var(--font-body);

  /* Body Small */
  --text-body-sm-size: 0.875rem;      /* 14px */
  --text-body-sm-weight: 400;
  --text-body-sm-line-height: 1.5;
  --text-body-sm-font: var(--font-body);

  /* Caption */
  --text-caption-size: 0.75rem;       /* 12px */
  --text-caption-weight: 500;
  --text-caption-line-height: 1.4;
  --text-caption-font: var(--font-body);

  /* Overline */
  --text-overline-size: 0.6875rem;    /* 11px */
  --text-overline-weight: 600;
  --text-overline-line-height: 1.4;
  --text-overline-letter-spacing: 0.08em;
  --text-overline-text-transform: uppercase;
  --text-overline-font: var(--font-body);
}
```

- [ ] **Step 2: Validate**

```bash
node -e "
const css = require('fs').readFileSync('design-system/tokens/typography.css', 'utf8');
const fonts = ['Manrope', 'Inter', 'JetBrains Mono'];
fonts.forEach(f => console.log(f + ':', css.includes(f) ? 'OK' : 'MISSING'));
const scales = ['display', 'h1', 'h2', 'h3', 'h4', 'body-lg', 'body', 'body-sm', 'caption', 'overline'];
scales.forEach(s => console.log('--text-' + s + '-size:', css.includes('--text-' + s + '-size') ? 'OK' : 'MISSING'));
"
```

Expected: All fonts OK, all scale levels OK.

- [ ] **Step 3: Commit**

```bash
git add design-system/tokens/typography.css
git commit -m "feat: add typography tokens with Manrope + Inter type scale"
```

---

## Task 4: Spacing tokens

**Files:**
- Create: `design-system/tokens/spacing.css`

- [ ] **Step 1: Write spacing.css**

```css
/* design-system/tokens/spacing.css
 * Quorant spacing system — 8px base, radii, breakpoints
 */

:root {
  /* === SPACING SCALE === */
  --space-1:  4px;
  --space-2:  8px;
  --space-3:  12px;
  --space-4:  16px;
  --space-5:  20px;
  --space-6:  24px;
  --space-8:  32px;
  --space-10: 40px;
  --space-12: 48px;
  --space-16: 64px;
  --space-20: 80px;

  /* === BORDER RADIUS === */
  --radius-sm:   4px;
  --radius-md:   8px;
  --radius-lg:   12px;
  --radius-full: 9999px;

  /* === BREAKPOINTS (for reference — use in media queries) === */
  --bp-sm:  640px;
  --bp-md:  768px;
  --bp-lg:  1024px;
  --bp-xl:  1280px;
  --bp-2xl: 1536px;

  /* === LAYOUT === */
  --container-max-width: 1280px;
  --sidebar-width: 230px;
  --member-max-width: 428px;
  --grid-gap: var(--space-4);
  --bottombar-height: 56px;
  --input-height: 44px;
  --touch-target-min: 44px;
  --tap-spacing-min: 8px;
}
```

- [ ] **Step 2: Commit**

```bash
git add design-system/tokens/spacing.css
git commit -m "feat: add spacing tokens with 8px base, radii, and layout values"
```

---

## Task 5: Elevation tokens

**Files:**
- Create: `design-system/tokens/elevation.css`

- [ ] **Step 1: Write elevation.css**

```css
/* design-system/tokens/elevation.css
 * Quorant elevation system — warm shadows (light), strong shadows (dark)
 */

:root {
  --shadow-xs:    0 1px 2px rgba(26, 26, 26, 0.05);
  --shadow-sm:    0 1px 3px rgba(26, 26, 26, 0.08), 0 1px 2px rgba(26, 26, 26, 0.04);
  --shadow-md:    0 4px 6px rgba(26, 26, 26, 0.07), 0 2px 4px rgba(26, 26, 26, 0.04);
  --shadow-lg:    0 10px 15px rgba(26, 26, 26, 0.08), 0 4px 6px rgba(26, 26, 26, 0.04);
  --shadow-xl:    0 20px 25px rgba(26, 26, 26, 0.08), 0 8px 10px rgba(26, 26, 26, 0.03);
  --shadow-2xl:   0 25px 50px rgba(26, 26, 26, 0.15);
  --shadow-inner: inset 0 2px 4px rgba(26, 26, 26, 0.05);
}

@media (prefers-color-scheme: dark) {
  :root {
    --shadow-xs:    0 1px 2px rgba(0, 0, 0, 0.2);
    --shadow-sm:    0 1px 3px rgba(0, 0, 0, 0.25), 0 1px 2px rgba(0, 0, 0, 0.15);
    --shadow-md:    0 4px 6px rgba(0, 0, 0, 0.25), 0 2px 4px rgba(0, 0, 0, 0.15);
    --shadow-lg:    0 10px 15px rgba(0, 0, 0, 0.3), 0 4px 6px rgba(0, 0, 0, 0.15);
    --shadow-xl:    0 20px 25px rgba(0, 0, 0, 0.3), 0 8px 10px rgba(0, 0, 0, 0.12);
    --shadow-2xl:   0 25px 50px rgba(0, 0, 0, 0.4);
    --shadow-inner: inset 0 2px 4px rgba(0, 0, 0, 0.2);
  }
}

[data-theme="dark"] {
  --shadow-xs:    0 1px 2px rgba(0, 0, 0, 0.2);
  --shadow-sm:    0 1px 3px rgba(0, 0, 0, 0.25), 0 1px 2px rgba(0, 0, 0, 0.15);
  --shadow-md:    0 4px 6px rgba(0, 0, 0, 0.25), 0 2px 4px rgba(0, 0, 0, 0.15);
  --shadow-lg:    0 10px 15px rgba(0, 0, 0, 0.3), 0 4px 6px rgba(0, 0, 0, 0.15);
  --shadow-xl:    0 20px 25px rgba(0, 0, 0, 0.3), 0 8px 10px rgba(0, 0, 0, 0.12);
  --shadow-2xl:   0 25px 50px rgba(0, 0, 0, 0.4);
  --shadow-inner: inset 0 2px 4px rgba(0, 0, 0, 0.2);
}
```

- [ ] **Step 2: Commit**

```bash
git add design-system/tokens/elevation.css
git commit -m "feat: add elevation tokens with warm light and dark shadows"
```

---

## Task 6: Motion tokens

**Files:**
- Create: `design-system/tokens/motion.css`

- [ ] **Step 1: Write motion.css**

```css
/* design-system/tokens/motion.css
 * Quorant motion system — easings, durations, keyframe animations
 * Behavior IS the brand. No decorative motifs — personality lives here.
 */

:root {
  /* === EASING CURVES === */
  --ease-default: cubic-bezier(0.25, 0.1, 0.25, 1);
  --ease-out:     cubic-bezier(0.0, 0.0, 0.2, 1);
  --ease-in:      cubic-bezier(0.4, 0.0, 1, 1);
  --ease-spring:  cubic-bezier(0.34, 1.56, 0.64, 1);
  --ease-smooth:  cubic-bezier(0.4, 0.0, 0.2, 1);

  /* === DURATIONS === */
  --duration-instant:  100ms;
  --duration-fast:     150ms;
  --duration-normal:   200ms;
  --duration-moderate: 300ms;
  --duration-slow:     400ms;
  --duration-stagger:  60ms;
}

/* === KEYFRAME ANIMATIONS === */

/* Stagger reveal — member app (rise up) */
@keyframes stagger-up {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Stagger reveal — management (slide from left) */
@keyframes stagger-left {
  from {
    opacity: 0;
    transform: translateX(-6px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

/* Conversational card hero entrance */
@keyframes hero-entrance {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Page content enter */
@keyframes page-enter {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Page content exit */
@keyframes page-exit {
  from {
    opacity: 1;
    transform: translateY(0);
  }
  to {
    opacity: 0;
    transform: translateY(-4px);
  }
}

/* Progress bar fill */
@keyframes fill-progress {
  from { width: 0; }
}

/* Live pulse dot */
@keyframes pulse-dot {
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.4;
    transform: scale(1.3);
  }
}

/* === REDUCED MOTION === */
@media (prefers-reduced-motion: reduce) {
  :root {
    --duration-instant:  100ms;
    --duration-fast:     100ms;
    --duration-normal:   100ms;
    --duration-moderate: 100ms;
    --duration-slow:     100ms;
    --duration-stagger:  0ms;
  }

  *, *::before, *::after {
    animation-duration: 100ms !important;
    animation-delay: 0ms !important;
    transition-duration: 100ms !important;
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add design-system/tokens/motion.css
git commit -m "feat: add motion tokens with easings, durations, and keyframes"
```

---

## Task 7: JSON tokens for React Native / mobile

**Files:**
- Create: `design-system/tokens/tokens.json`

- [ ] **Step 1: Write tokens.json**

```json
{
  "color": {
    "light": {
      "primary": {
        "900": "#0A3D4F", "800": "#0E4F65", "700": "#13647E",
        "600": "#1A7A9A", "500": "#2391B4", "400": "#3DACC9",
        "300": "#6FC4DA", "200": "#A3DAE8", "100": "#D1EDF4", "50": "#EBF7FB"
      },
      "secondary": {
        "600": "#B8860B", "500": "#D4A017", "400": "#E8BA3C",
        "200": "#F5DFA0", "100": "#FBF0D1", "50": "#FDF8EB"
      },
      "neutral": {
        "950": "#1A1A1A", "900": "#2D2D2D", "700": "#525252",
        "500": "#7A7A7A", "400": "#A3A3A3", "200": "#E0E0E0",
        "100": "#F0F0F0", "50": "#F8F8F8", "white": "#FFFFFF"
      },
      "success": { "600": "#16794A", "100": "#D4EDDA" },
      "error":   { "600": "#C0392B", "100": "#F8D7DA" },
      "warning": { "600": "#D48806", "100": "#FFF3CD" },
      "info":    { "600": "#1A7A9A", "100": "#D1EDF4" },
      "surface": {
        "page": "#F8F8F8", "card": "#FFFFFF", "elevated": "#FFFFFF"
      }
    },
    "dark": {
      "primary": {
        "600": "#3DACC9", "500": "#5BBDD6", "400": "#7ACEE3",
        "300": "#A3DAE8", "200": "#13647E", "100": "#0E4F65", "50": "#0A3D4F"
      },
      "secondary": { "500": "#E8BA3C" },
      "neutral": {
        "950": "#F0F0F0", "900": "#E0E0E0", "700": "#A3A3A3",
        "500": "#7A7A7A", "400": "#525252", "200": "#333333",
        "100": "#262626", "50": "#1E1E1E", "white": "#2A2A2A"
      },
      "success": { "600": "#34C77A", "100": "#1A3D2A" },
      "error":   { "600": "#E85C4A", "100": "#3D1A1A" },
      "warning": { "600": "#E8BA3C", "100": "#3D2E0A" },
      "info":    { "600": "#3DACC9", "100": "#0E4F65" },
      "surface": {
        "page": "#141414", "card": "#1E1E1E", "elevated": "#2A2A2A"
      }
    }
  },
  "typography": {
    "fontFamily": {
      "heading": "Manrope",
      "body": "Inter",
      "mono": "JetBrains Mono"
    },
    "scale": {
      "display":  { "size": 48, "weight": 800, "lineHeight": 1.1, "font": "heading" },
      "h1":       { "size": 36, "weight": 700, "lineHeight": 1.2, "font": "heading" },
      "h2":       { "size": 28, "weight": 700, "lineHeight": 1.25, "font": "heading" },
      "h3":       { "size": 22, "weight": 600, "lineHeight": 1.3, "font": "heading" },
      "h4":       { "size": 18, "weight": 600, "lineHeight": 1.35, "font": "heading" },
      "bodyLg":   { "size": 18, "weight": 400, "lineHeight": 1.6, "font": "body" },
      "body":     { "size": 16, "weight": 400, "lineHeight": 1.6, "font": "body" },
      "bodySm":   { "size": 14, "weight": 400, "lineHeight": 1.5, "font": "body" },
      "caption":  { "size": 12, "weight": 500, "lineHeight": 1.4, "font": "body" },
      "overline": { "size": 11, "weight": 600, "lineHeight": 1.4, "font": "body", "letterSpacing": 0.08, "textTransform": "uppercase" }
    }
  },
  "spacing": {
    "1": 4, "2": 8, "3": 12, "4": 16, "5": 20,
    "6": 24, "8": 32, "10": 40, "12": 48, "16": 64, "20": 80
  },
  "radius": {
    "sm": 4, "md": 8, "lg": 12, "full": 9999
  },
  "shadow": {
    "light": {
      "xs": "0 1px 2px rgba(26,26,26,0.05)",
      "sm": "0 1px 3px rgba(26,26,26,0.08), 0 1px 2px rgba(26,26,26,0.04)",
      "md": "0 4px 6px rgba(26,26,26,0.07), 0 2px 4px rgba(26,26,26,0.04)",
      "lg": "0 10px 15px rgba(26,26,26,0.08), 0 4px 6px rgba(26,26,26,0.04)",
      "xl": "0 20px 25px rgba(26,26,26,0.08), 0 8px 10px rgba(26,26,26,0.03)",
      "2xl": "0 25px 50px rgba(26,26,26,0.15)",
      "inner": "inset 0 2px 4px rgba(26,26,26,0.05)"
    },
    "dark": {
      "xs": "0 1px 2px rgba(0,0,0,0.2)",
      "sm": "0 1px 3px rgba(0,0,0,0.25), 0 1px 2px rgba(0,0,0,0.15)",
      "md": "0 4px 6px rgba(0,0,0,0.25), 0 2px 4px rgba(0,0,0,0.15)",
      "lg": "0 10px 15px rgba(0,0,0,0.3), 0 4px 6px rgba(0,0,0,0.15)",
      "xl": "0 20px 25px rgba(0,0,0,0.3), 0 8px 10px rgba(0,0,0,0.12)",
      "2xl": "0 25px 50px rgba(0,0,0,0.4)",
      "inner": "inset 0 2px 4px rgba(0,0,0,0.2)"
    }
  },
  "motion": {
    "easing": {
      "default": "cubic-bezier(0.25, 0.1, 0.25, 1)",
      "out": "cubic-bezier(0.0, 0.0, 0.2, 1)",
      "in": "cubic-bezier(0.4, 0.0, 1, 1)",
      "spring": "cubic-bezier(0.34, 1.56, 0.64, 1)",
      "smooth": "cubic-bezier(0.4, 0.0, 0.2, 1)"
    },
    "duration": {
      "instant": 100, "fast": 150, "normal": 200,
      "moderate": 300, "slow": 400, "stagger": 60
    }
  },
  "breakpoint": {
    "sm": 640, "md": 768, "lg": 1024, "xl": 1280, "2xl": 1536
  }
}
```

- [ ] **Step 2: Validate JSON is parseable**

```bash
node -e "const t = JSON.parse(require('fs').readFileSync('design-system/tokens/tokens.json','utf8')); console.log('Sections:', Object.keys(t).join(', ')); console.log('Light colors:', Object.keys(t.color.light).length, 'groups'); console.log('Dark colors:', Object.keys(t.color.dark).length, 'groups'); console.log('Type scales:', Object.keys(t.typography.scale).length);"
```

Expected: 6 sections, 7+ light color groups, 7+ dark color groups, 10 type scales.

- [ ] **Step 3: Commit**

```bash
git add design-system/tokens/tokens.json
git commit -m "feat: add JSON tokens for React Native / mobile consumption"
```

---

## Task 8: Component browser shell

**Files:**
- Create: `design-system/component-browser.html`

This is the Storybook-like frame that loads component pages in an iframe with a dark mode toggle and sidebar navigation.

- [ ] **Step 1: Write component-browser.html**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Quorant Design System — Component Browser</title>
  <link rel="stylesheet" href="tokens/colors.css">
  <link rel="stylesheet" href="tokens/typography.css">
  <link rel="stylesheet" href="tokens/spacing.css">
  <link rel="stylesheet" href="tokens/elevation.css">
  <link rel="stylesheet" href="tokens/motion.css">
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    html, body { height: 100%; overflow: hidden; }
    body {
      font-family: var(--font-body);
      background: var(--surface-page);
      color: var(--color-neutral-950);
      display: flex;
    }

    /* Sidebar */
    .browser-sidebar {
      width: 240px;
      background: var(--surface-card);
      border-right: 1px solid var(--color-neutral-200);
      display: flex;
      flex-direction: column;
      flex-shrink: 0;
      height: 100vh;
    }

    .browser-brand {
      padding: var(--space-5) var(--space-4);
      border-bottom: 1px solid var(--color-neutral-200);
    }
    .browser-brand h1 {
      font-family: var(--font-heading);
      font-size: var(--text-h4-size);
      font-weight: 700;
      color: var(--color-primary-600);
    }
    .browser-brand span {
      font-size: var(--text-caption-size);
      color: var(--color-neutral-500);
    }

    .browser-controls {
      padding: var(--space-3) var(--space-4);
      border-bottom: 1px solid var(--color-neutral-200);
      display: flex;
      align-items: center;
      gap: var(--space-2);
    }
    .theme-toggle {
      display: flex;
      align-items: center;
      gap: var(--space-2);
      font-size: var(--text-caption-size);
      color: var(--color-neutral-700);
      cursor: pointer;
      user-select: none;
    }
    .theme-toggle-track {
      width: 36px; height: 20px;
      background: var(--color-neutral-400);
      border-radius: var(--radius-full);
      position: relative;
      transition: background var(--duration-fast) var(--ease-default);
    }
    .theme-toggle-track.active {
      background: var(--color-primary-600);
    }
    .theme-toggle-thumb {
      width: 16px; height: 16px;
      background: var(--color-white);
      border-radius: 50%;
      position: absolute;
      top: 2px; left: 2px;
      transition: transform var(--duration-fast) var(--ease-default);
      box-shadow: var(--shadow-xs);
    }
    .theme-toggle-track.active .theme-toggle-thumb {
      transform: translateX(16px);
    }

    .browser-nav {
      flex: 1;
      overflow-y: auto;
      padding: var(--space-3) 0;
    }
    .nav-section-label {
      font-size: var(--text-overline-size);
      font-weight: var(--text-overline-weight);
      text-transform: var(--text-overline-text-transform);
      letter-spacing: var(--text-overline-letter-spacing);
      color: var(--color-neutral-500);
      padding: var(--space-3) var(--space-4) var(--space-1);
    }
    .nav-item {
      display: block;
      padding: var(--space-2) var(--space-4);
      font-size: var(--text-body-sm-size);
      color: var(--color-neutral-700);
      text-decoration: none;
      cursor: pointer;
      transition: background var(--duration-fast) var(--ease-default),
                  color var(--duration-fast) var(--ease-default);
    }
    .nav-item:hover {
      background: var(--color-neutral-100);
      color: var(--color-neutral-950);
    }
    .nav-item.active {
      background: var(--color-primary-50);
      color: var(--color-primary-600);
      font-weight: 600;
    }

    /* Main content area */
    .browser-main {
      flex: 1;
      display: flex;
      flex-direction: column;
    }
    .browser-toolbar {
      padding: var(--space-2) var(--space-4);
      background: var(--surface-card);
      border-bottom: 1px solid var(--color-neutral-200);
      font-size: var(--text-caption-size);
      color: var(--color-neutral-500);
      display: flex;
      align-items: center;
      gap: var(--space-2);
    }
    .browser-toolbar .current-page {
      color: var(--color-neutral-950);
      font-weight: 600;
    }
    .browser-iframe-wrapper {
      flex: 1;
      overflow: hidden;
    }
    .browser-iframe-wrapper iframe {
      width: 100%;
      height: 100%;
      border: none;
    }
  </style>
</head>
<body>

<aside class="browser-sidebar">
  <div class="browser-brand">
    <h1>Quorant</h1>
    <span>Design System v1.0</span>
  </div>

  <div class="browser-controls">
    <div class="theme-toggle" onclick="toggleTheme()">
      <div class="theme-toggle-track" id="theme-track">
        <div class="theme-toggle-thumb"></div>
      </div>
      <span id="theme-label">Light</span>
    </div>
  </div>

  <nav class="browser-nav">
    <div class="nav-section-label">Overview</div>
    <a class="nav-item active" onclick="loadPage('guide.html', this)">Design Guide</a>

    <div class="nav-section-label">Components</div>
    <a class="nav-item" onclick="loadPage('components/buttons.html', this)">Buttons</a>
    <a class="nav-item" onclick="loadPage('components/cards.html', this)">Cards</a>
    <a class="nav-item" onclick="loadPage('components/forms.html', this)">Forms</a>
    <a class="nav-item" onclick="loadPage('components/badges.html', this)">Badges</a>
    <a class="nav-item" onclick="loadPage('components/alerts.html', this)">Alerts</a>
    <a class="nav-item" onclick="loadPage('components/tables.html', this)">Tables</a>
    <a class="nav-item" onclick="loadPage('components/navigation.html', this)">Navigation</a>
    <a class="nav-item" onclick="loadPage('components/modals.html', this)">Modals</a>
    <a class="nav-item" onclick="loadPage('components/feed.html', this)">Feed / Pulse</a>

    <div class="nav-section-label">Patterns</div>
    <a class="nav-item" onclick="loadPage('patterns/conversational-ui.html', this)">Conversational UI</a>
    <a class="nav-item" onclick="loadPage('patterns/motion-specs.html', this)">Motion Specs</a>
    <a class="nav-item" onclick="loadPage('patterns/management-layouts.html', this)">Management Layouts</a>
    <a class="nav-item" onclick="loadPage('patterns/member-layouts.html', this)">Member Layouts</a>
  </nav>
</aside>

<main class="browser-main">
  <div class="browser-toolbar">
    <span>Viewing:</span>
    <span class="current-page" id="current-page">Design Guide</span>
  </div>
  <div class="browser-iframe-wrapper">
    <iframe id="preview-frame" src="guide.html"></iframe>
  </div>
</main>

<script>
  let isDark = false;

  function toggleTheme() {
    isDark = !isDark;
    const theme = isDark ? 'dark' : 'light';

    // Toggle on browser shell
    document.documentElement.setAttribute('data-theme', isDark ? 'dark' : '');

    // Toggle on iframe content
    const iframe = document.getElementById('preview-frame');
    if (iframe.contentDocument) {
      iframe.contentDocument.documentElement.setAttribute('data-theme', isDark ? 'dark' : '');
    }

    // Update toggle UI
    document.getElementById('theme-track').classList.toggle('active', isDark);
    document.getElementById('theme-label').textContent = isDark ? 'Dark' : 'Light';
  }

  function loadPage(url, navItem) {
    // Update iframe
    const iframe = document.getElementById('preview-frame');
    iframe.src = url;

    // Apply current theme to new page once loaded
    iframe.onload = function() {
      if (isDark && iframe.contentDocument) {
        iframe.contentDocument.documentElement.setAttribute('data-theme', 'dark');
      }
    };

    // Update nav active state
    document.querySelectorAll('.nav-item').forEach(item => item.classList.remove('active'));
    navItem.classList.add('active');

    // Update toolbar
    document.getElementById('current-page').textContent = navItem.textContent;
  }
</script>

</body>
</html>
```

- [ ] **Step 2: Verify it opens in a browser**

```bash
echo "Open design-system/component-browser.html in Chrome"
echo "Expected: sidebar with nav, dark mode toggle, iframe area (will show 404 until guide.html exists)"
```

- [ ] **Step 3: Commit**

```bash
git add design-system/component-browser.html
git commit -m "feat: add component browser shell with dark mode toggle and nav"
```

---

## Task 9: Guide overview page

**Files:**
- Create: `design-system/guide.html`

This is the landing page shown in the component browser iframe. It provides a visual overview of the design system.

- [ ] **Step 1: Write guide.html**

This file imports all token CSS files, renders color swatches, type specimens, spacing scale, shadow samples, and motion previews. It serves as both the component browser landing page and a standalone reference.

The file should be a complete HTML document that:
- Imports all 5 token CSS files via `<link>` tags
- Shows a "Brand" section with personality statement
- Shows color swatches for all palette groups (primary, secondary, neutral, semantic) with hex values and token names
- Shows typography specimens at every scale level with actual Manrope/Inter rendering
- Shows spacing scale as visual bars
- Shows shadow cards at each elevation level
- Shows motion easing curve previews (animated dots moving along paths)
- Has a `[data-theme="dark"]` toggle button so it works standalone too
- Uses semantic HTML sections with IDs for deep linking

Full implementation: this file will be ~400 lines. The implementing agent should build it section by section, importing tokens and rendering live examples of each token category. Each color swatch renders as a `<div>` with the background set via the CSS custom property, displaying the token name and hex value. Typography specimens render headings and body text at each scale level. Shadow cards show boxes at each elevation. Motion section shows animated examples of the signature patterns.

- [ ] **Step 2: Verify in browser**

Open `design-system/component-browser.html` — the guide should render in the iframe. Toggle dark mode — all swatches, specimens, and examples should update.

- [ ] **Step 3: Commit**

```bash
git add design-system/guide.html
git commit -m "feat: add design guide overview page with token visualization"
```

---

## Task 10: Buttons component page

**Files:**
- Create: `design-system/components/buttons.html`

- [ ] **Step 1: Write buttons.html**

Complete HTML page importing all token CSS files. Shows:

- **4 variants** (Primary, Secondary, Ghost, Destructive) in a row
- **3 sizes** (Small, Medium, Large) for each variant
- **States** for the Primary button: default, hover (simulated with class), active (simulated), focus, disabled
- **App divergence section**: Member "Pay Now" with gradient + elevated shadow vs. Management flat primary
- All buttons use token variables for colors, radii, shadows, typography
- Hover/active states use the motion token durations and easings
- Each example has a label showing the variant name, size, and relevant CSS

- [ ] **Step 2: Verify in component browser**

Load via sidebar nav → Buttons. Toggle dark mode. All variants should update colors correctly.

- [ ] **Step 3: Commit**

```bash
git add design-system/components/buttons.html
git commit -m "feat: add buttons component page with all variants and states"
```

---

## Task 11: Cards component page

**Files:**
- Create: `design-system/components/cards.html`

- [ ] **Step 1: Write cards.html**

Shows all 5 card types from the spec:
- **Default card** with hover lift animation
- **Stat card** with colored top border (show 4 colors)
- **Conversational card** with body-lg text and hero entrance animation
- **Feed item** with avatar + text + meta + stagger entrance
- **Interactive card** with pointer cursor and hover response

Use HOA-themed content: "Oakridge Commons is in good shape this month" for conversational, resident data for stat cards, community updates for feed items.

- [ ] **Step 2: Verify in component browser**

Cards should animate on page load (stagger). Hover should lift cards. Dark mode should update all surfaces and text.

- [ ] **Step 3: Commit**

```bash
git add design-system/components/cards.html
git commit -m "feat: add cards component page with all 5 types and animations"
```

---

## Task 12: Forms component page

**Files:**
- Create: `design-system/components/forms.html`

- [ ] **Step 1: Write forms.html**

Shows all form input variants:
- **Text input**: default, focus (with ring), error (with message), disabled
- **Textarea**: default, with character count
- **Select**: custom-styled dropdown
- **Checkbox**: default, checked, disabled
- **Radio**: default, selected, disabled
- **Toggle switch**: off, on, disabled
- **Input with label**: showing proper label styling (Inter 500, 14px)
- **Form group example**: a realistic "Submit Maintenance Request" form with multiple field types

All inputs use `--input-height: 44px`, `--radius-md`, token colors for borders/focus.

- [ ] **Step 2: Verify focus states work with keyboard navigation**

Tab through the form — every input should show the primary-600 border + primary-200 ring on `:focus-visible`.

- [ ] **Step 3: Commit**

```bash
git add design-system/components/forms.html
git commit -m "feat: add forms component page with all input variants and states"
```

---

## Task 13: Badges component page

**Files:**
- Create: `design-system/components/badges.html`

- [ ] **Step 1: Write badges.html**

Shows:
- **Status pills**: Current (green), Past Due (yellow), Delinquent (red), Grace Period (teal)
- **Category tags**: Amenity, Meeting, Notice, Update, Request
- **Count badges**: numbers 1-99 on secondary-500 background
- **Size comparison**: all badges at their specified sizes
- **In-context examples**: badges inside a mock table row and inside a mock feed item

- [ ] **Step 2: Commit**

```bash
git add design-system/components/badges.html
git commit -m "feat: add badges component page with pills, tags, and count badges"
```

---

## Task 14: Alerts component page

**Files:**
- Create: `design-system/components/alerts.html`

- [ ] **Step 1: Write alerts.html**

Shows all 4 semantic alert types:
- **Success**: "Payment of $285.00 received successfully."
- **Error**: "Payment failed. Please check your payment method and try again."
- **Warning**: "Your March assessment is past due. A late fee will apply after March 20th."
- **Info**: "Board meeting scheduled for April 15th at 7:00 PM."

Each alert: left border, semantic bg, icon area, message text, optional action link. Dark mode: bg shifts to semantic-600 at 10% opacity.

- [ ] **Step 2: Commit**

```bash
git add design-system/components/alerts.html
git commit -m "feat: add alerts component page with all semantic types"
```

---

## Task 15: Tables component page

**Files:**
- Create: `design-system/components/tables.html`

- [ ] **Step 1: Write tables.html**

Shows the management-style data table:
- Column headers with uppercase styling
- Avatar cells (initials in colored circles + name)
- Amount cells in JetBrains Mono (negative in red, zero in green)
- Status pill cells
- Alternating row backgrounds
- Hover highlighting
- 5-6 rows of realistic HOA resident data (from our mockups)
- Sortable column header indicators (visual only)

- [ ] **Step 2: Commit**

```bash
git add design-system/components/tables.html
git commit -m "feat: add tables component page with management-style data table"
```

---

## Task 16: Navigation component page

**Files:**
- Create: `design-system/components/navigation.html`

- [ ] **Step 1: Write navigation.html**

Shows all navigation patterns:
- **Management sidebar**: full-height mock with logo, section labels, nav items (active state), badges, user profile at bottom
- **Member bottom tab bar**: 5-item bar with icons and labels, active state
- **Breadcrumbs**: "Properties / Oakridge Commons / Residents"
- **Tabs**: horizontal tab bar with active tab indicator

Each rendered at realistic size with token-correct colors and spacing.

- [ ] **Step 2: Commit**

```bash
git add design-system/components/navigation.html
git commit -m "feat: add navigation component page with sidebar, tabs, breadcrumbs"
```

---

## Task 17: Modals component page

**Files:**
- Create: `design-system/components/modals.html`

- [ ] **Step 1: Write modals.html**

Shows:
- **Dialog**: "Confirm Payment" with message, Cancel + Confirm buttons
- **Confirmation (destructive)**: "Delete Violation Record?" with Cancel + Delete (destructive button)
- **Drawer**: side panel sliding in from right

Each with:
- Backdrop overlay at correct opacity
- Entry animation (translateY + opacity)
- Correct shadow, radius, padding from tokens
- A "Show Modal" button to trigger each one via JavaScript

- [ ] **Step 2: Verify modal animations work**

Click each trigger button. Modal should animate in with `400ms ease-out`. Click backdrop to dismiss.

- [ ] **Step 3: Commit**

```bash
git add design-system/components/modals.html
git commit -m "feat: add modals component page with dialog, confirmation, and drawer"
```

---

## Task 18: Feed / Community Pulse component page

**Files:**
- Create: `design-system/components/feed.html`

This is the signature component — the most important page in the design system.

- [ ] **Step 1: Write feed.html**

Shows side-by-side:

**Management: Recent Activity Stream**
- Header with "Recent Activity", live pulse dot, filter tabs (All, Payments, Violations, Requests)
- 5 stream items with: initials avatar (32px, colored), factual narrative text ("Sarah Chen paid April assessment — $285.00"), unit + timestamp meta, action link or status pill
- Stagger entrance: `translateX(-6px)`, 60ms stagger between items

**Member: Community Pulse**
- Header with "Community Pulse", live pulse dot
- 4 pulse items with: emoji/icon avatar (30px, gradient), conversational text ("The community pool opens May 1st"), timestamp + category tag
- Stagger entrance: `translateY(8px)`, 60ms stagger between items

**Conversational Status Card**
- Member variant: "Your April assessment of $285.00 is ready. Autopay handles it on the 12th."
- Management variant: "Oakridge Commons is in good shape this month. Collections at 94.2% — up from last month."
- Hero entrance animation: `translateY(16px)`, 500ms

Include a "Replay Animations" button that re-triggers all entrance animations.

- [ ] **Step 2: Verify animations**

Page load should show stagger reveals. Click "Replay Animations" to see them again. Live pulse dot should animate continuously. Dark mode should update all colors.

- [ ] **Step 3: Commit**

```bash
git add design-system/components/feed.html
git commit -m "feat: add feed/pulse component page — the signature Quorant component"
```

---

## Task 19: Conversational UI pattern page

**Files:**
- Create: `design-system/patterns/conversational-ui.html`

- [ ] **Step 1: Write conversational-ui.html**

Documents the conversational UI guidelines with live examples:
- **Principle statement**: "Status should feel like a helpful neighbor, not an accounting system."
- **Comparison table**: traditional vs. Quorant language for each context (balance due, current, past due, payment received, work order)
- **Rules list**: the 6 rules from the spec, each with a do/don't example rendered as cards
- **Escalation scale**: visual showing calm → nudge → direct language with color coding
- **Member vs. Management voice comparison**: side-by-side cards showing same event in both tones

- [ ] **Step 2: Commit**

```bash
git add design-system/patterns/conversational-ui.html
git commit -m "feat: add conversational UI pattern page with guidelines and examples"
```

---

## Task 20: Motion specs pattern page

**Files:**
- Create: `design-system/patterns/motion-specs.html`

- [ ] **Step 1: Write motion-specs.html**

Interactive reference for all motion patterns:
- **Easing curves**: 5 animated dots moving along a track, each using a different easing curve, with the curve name and CSS value labeled
- **Duration scale**: bars that fill at each duration speed
- **Signature patterns**: each of the 6 patterns from the spec with a "Play" button:
  1. Staggered Reveal (member + management variants)
  2. Card Depth Response (hover demo)
  3. Progress & Status (animated progress bar + pulse dot)
  4. Button Micro-interactions (hover/active/focus demo)
  5. Page Transitions (simulated view switch)
  6. Conversational Card Entrance (hero card animating in)
- **Reduced motion section**: toggle to simulate `prefers-reduced-motion` and show how each pattern degrades

- [ ] **Step 2: Verify all animations play correctly**

Click each "Play" button. Animations should match the spec timings and easings.

- [ ] **Step 3: Commit**

```bash
git add design-system/patterns/motion-specs.html
git commit -m "feat: add motion specs pattern page with interactive animation demos"
```

---

## Task 21: Management layouts pattern page

**Files:**
- Create: `design-system/patterns/management-layouts.html`

- [ ] **Step 1: Write management-layouts.html**

Shows wireframe-level layout patterns for the management portal:
- **Dashboard layout**: conversational summary → 4-col stat grid → 2-col (activity stream + charts)
- **List view layout**: filter bar → data table → pagination
- **Detail view layout**: entity header + status badge → tabbed content → action sidebar
- **Form view layout**: stepped form with progress indicator
- **Empty state**: "No residents added yet. Import a CSV or add them manually."

Each rendered as a wireframe mockup using actual tokens (colors, spacing, radii) but with placeholder content areas.

- [ ] **Step 2: Commit**

```bash
git add design-system/patterns/management-layouts.html
git commit -m "feat: add management layout patterns with dashboard, list, detail, form"
```

---

## Task 22: Member layouts pattern page

**Files:**
- Create: `design-system/patterns/member-layouts.html`

- [ ] **Step 1: Write member-layouts.html**

Shows wireframe-level layout patterns for the member app (428px max-width):
- **Home layout**: header greeting → conversational card → quick actions (4-col) → community pulse → upcoming events
- **Payment flow**: balance → method select → confirm → success state
- **Request flow**: category → form → photo upload → confirmation
- **Detail view**: status header → update timeline → action buttons
- **Empty states**: "You're all caught up", "It's quiet in the neighborhood today", "Clean record"

Each rendered as a phone-width mockup using actual tokens.

- [ ] **Step 2: Commit**

```bash
git add design-system/patterns/member-layouts.html
git commit -m "feat: add member app layout patterns with home, payment, request flows"
```

---

## Task 23: Final integration and validation

**Files:**
- Modify: `design-system/component-browser.html` (update nav links if needed)

- [ ] **Step 1: Verify all nav links in component browser work**

Open `design-system/component-browser.html` and click every link in the sidebar. Each should load the correct page in the iframe.

- [ ] **Step 2: Test dark mode toggle across all pages**

Toggle dark mode on each page. Verify:
- All text remains readable
- All backgrounds update
- All component colors update
- Shadows strengthen appropriately
- No elements "disappear" against dark backgrounds

- [ ] **Step 3: Test reduced motion**

In Chrome DevTools → Rendering → check "Emulate CSS media feature prefers-reduced-motion: reduce". Verify:
- All animations collapse to instant
- Stagger delays become 0
- Pulse dot becomes static
- Everything remains functional

- [ ] **Step 4: Verify keyboard accessibility**

Tab through buttons.html, forms.html, and modals.html. Verify:
- All interactive elements receive focus
- Focus rings (3px primary-300) are visible
- Tab order follows visual order

- [ ] **Step 5: Final commit**

```bash
git add -A design-system/
git commit -m "feat: complete Quorant design system v1.0

Full design system package with:
- Token files (CSS + JSON) for colors, typography, spacing, elevation, motion
- 9 component pages with light/dark mode and interaction demos
- 4 pattern pages covering conversational UI, motion, and layouts
- Storybook-like component browser with dark mode toggle
- WCAG AA compliant, prefers-reduced-motion aware"
```

---

## Summary

| Task | Description | Key Files |
|---|---|---|
| 1 | Scaffolding + .gitignore | `.gitignore`, dirs |
| 2 | Color tokens | `tokens/colors.css` |
| 3 | Typography tokens | `tokens/typography.css` |
| 4 | Spacing tokens | `tokens/spacing.css` |
| 5 | Elevation tokens | `tokens/elevation.css` |
| 6 | Motion tokens | `tokens/motion.css` |
| 7 | JSON tokens | `tokens/tokens.json` |
| 8 | Component browser shell | `component-browser.html` |
| 9 | Guide overview | `guide.html` |
| 10 | Buttons | `components/buttons.html` |
| 11 | Cards | `components/cards.html` |
| 12 | Forms | `components/forms.html` |
| 13 | Badges | `components/badges.html` |
| 14 | Alerts | `components/alerts.html` |
| 15 | Tables | `components/tables.html` |
| 16 | Navigation | `components/navigation.html` |
| 17 | Modals | `components/modals.html` |
| 18 | Feed / Pulse | `components/feed.html` |
| 19 | Conversational UI | `patterns/conversational-ui.html` |
| 20 | Motion specs | `patterns/motion-specs.html` |
| 21 | Management layouts | `patterns/management-layouts.html` |
| 22 | Member layouts | `patterns/member-layouts.html` |
| 23 | Integration + validation | All files |

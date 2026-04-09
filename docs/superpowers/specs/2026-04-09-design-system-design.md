# Quorant Design System — Specification

## Overview

Quorant is an HOA management platform with two separate applications:

- **Management Portal** (`manage.quorant.com`) — For management firms. Professional, data-dense, enterprise feel.
- **Member App** (mobile + web) — For homeowners and board members. Community-oriented, conversational. Board members get elevated features within it.

The two apps share a design foundation (typography, spacing, color tokens) but diverge in component density, interaction feel, and tone. The design system is the single source of truth for both.

### Brand Personality

Quorant's identity lives in **behavior, not decoration**. No signature shapes or decorative motifs. The personality comes from:

- Conversational UI — status told as narrative, not raw data
- Micro-interactions — cards that respond with depth, staggered reveals, progress animations
- Community-first layout — the member app leads with neighborhood activity, not account balances
- Motion rhythm — the way information appears and responds defines the brand

---

## Deliverable Structure

```
design-system/
├── tokens/
│   ├── colors.css              # Color custom properties (light + dark)
│   ├── typography.css           # Font families, scale, weights, line-heights
│   ├── spacing.css              # Spacing scale, radii, breakpoints
│   ├── elevation.css            # Shadows (light + dark)
│   ├── motion.css               # Transitions, easings, durations, keyframes
│   └── tokens.json              # All tokens in JSON for React Native / mobile
│
├── components/
│   ├── buttons.html             # All button variants × sizes × states
│   ├── cards.html               # Default, interactive, conversational, stat, feed
│   ├── forms.html               # Inputs, selects, checkboxes, radios, toggles
│   ├── badges.html              # Status pills, role badges, tags
│   ├── alerts.html              # Semantic alerts (success, error, warning, info)
│   ├── tables.html              # Data tables with sorting/filtering UI
│   ├── navigation.html          # Sidebar, bottom tab bar, breadcrumbs, tabs
│   ├── modals.html              # Dialog, confirmation, drawer
│   └── feed.html                # Activity stream, community pulse, conversational cards
│
├── patterns/
│   ├── conversational-ui.html   # Conversational status, contextual language guidelines
│   ├── motion-specs.html        # Interaction timing, easing curves, stagger patterns
│   ├── management-layouts.html  # Dashboard, table views, detail pages
│   └── member-layouts.html      # Home feed, payment flow, request flow
│
├── guide.html                   # Main entry — visual overview, links to all sections
└── component-browser.html       # Storybook-like browser with live light/dark toggle
```

---

## Color Tokens

### Light Mode

#### Primary (Teal)

| Token | Value | Usage |
|---|---|---|
| `--color-primary-900` | `#0A3D4F` | Sidebar bg, heading text |
| `--color-primary-800` | `#0E4F65` | Dark headers, footer |
| `--color-primary-700` | `#13647E` | Button hover |
| `--color-primary-600` | `#1A7A9A` | **Primary** — buttons, links, active states |
| `--color-primary-500` | `#2391B4` | Secondary button fills |
| `--color-primary-400` | `#3DACC9` | Lighter interactive elements |
| `--color-primary-300` | `#6FC4DA` | Focus rings, light accents |
| `--color-primary-200` | `#A3DAE8` | Selected rows, light borders |
| `--color-primary-100` | `#D1EDF4` | Info backgrounds, tints |
| `--color-primary-50` | `#EBF7FB` | Subtle background wash |

#### Secondary (Amber)

| Token | Value | Usage |
|---|---|---|
| `--color-secondary-600` | `#B8860B` | Dark accent, hover |
| `--color-secondary-500` | `#D4A017` | **Accent** — highlights, badges, dates |
| `--color-secondary-400` | `#E8BA3C` | Light accent interactions |
| `--color-secondary-200` | `#F5DFA0` | Accent backgrounds |
| `--color-secondary-100` | `#FBF0D1` | Subtle accent wash |
| `--color-secondary-50` | `#FDF8EB` | Lightest tint |

#### Neutral (Warm Gray)

| Token | Value | Usage |
|---|---|---|
| `--color-neutral-950` | `#1A1A1A` | Primary text |
| `--color-neutral-900` | `#2D2D2D` | Headings |
| `--color-neutral-700` | `#525252` | Secondary text |
| `--color-neutral-500` | `#7A7A7A` | Placeholder, disabled |
| `--color-neutral-400` | `#A3A3A3` | Borders |
| `--color-neutral-200` | `#E0E0E0` | Light borders, dividers |
| `--color-neutral-100` | `#F0F0F0` | Alt row backgrounds |
| `--color-neutral-50` | `#F8F8F8` | Page background |
| `--color-white` | `#FFFFFF` | Cards, inputs |

#### Semantic

| Token (foreground / background) | Values | Usage |
|---|---|---|
| `--color-success-600` / `-100` | `#16794A` / `#D4EDDA` | Confirmations, paid |
| `--color-error-600` / `-100` | `#C0392B` / `#F8D7DA` | Errors, delinquent |
| `--color-warning-600` / `-100` | `#D48806` / `#FFF3CD` | Warnings, past due |
| `--color-info-600` / `-100` | `#1A7A9A` / `#D1EDF4` | Informational |

#### Surfaces

| Token | Value |
|---|---|
| `--surface-page` | `#F8F8F8` |
| `--surface-card` | `#FFFFFF` |
| `--surface-elevated` | `#FFFFFF` |
| `--surface-sidebar` | `linear-gradient(180deg, #072D3B, #0A3D4F)` |
| `--surface-header` | `linear-gradient(160deg, #072D3B, #0E4F65, #1A7A9A)` |

### Dark Mode

| Token | Light Value | Dark Value |
|---|---|---|
| `--color-primary-600` | `#1A7A9A` | `#3DACC9` |
| `--color-primary-500` | `#2391B4` | `#5BBDD6` |
| `--color-primary-400` | `#3DACC9` | `#7ACEE3` |
| `--color-primary-300` | `#6FC4DA` | `#A3DAE8` |
| `--color-primary-200` | `#A3DAE8` | `#13647E` |
| `--color-primary-100` | `#D1EDF4` | `#0E4F65` |
| `--color-primary-50` | `#EBF7FB` | `#0A3D4F` |
| `--color-secondary-500` | `#D4A017` | `#E8BA3C` |
| `--color-neutral-950` | `#1A1A1A` | `#F0F0F0` |
| `--color-neutral-900` | `#2D2D2D` | `#E0E0E0` |
| `--color-neutral-700` | `#525252` | `#A3A3A3` |
| `--color-neutral-500` | `#7A7A7A` | `#7A7A7A` |
| `--color-neutral-400` | `#A3A3A3` | `#525252` |
| `--color-neutral-200` | `#E0E0E0` | `#333333` |
| `--color-neutral-100` | `#F0F0F0` | `#262626` |
| `--color-neutral-50` | `#F8F8F8` | `#1E1E1E` |
| `--color-white` | `#FFFFFF` | `#2A2A2A` |
| `--color-success-600` / `-100` | `#16794A` / `#D4EDDA` | `#34C77A` / `#1A3D2A` |
| `--color-error-600` / `-100` | `#C0392B` / `#F8D7DA` | `#E85C4A` / `#3D1A1A` |
| `--color-warning-600` / `-100` | `#D48806` / `#FFF3CD` | `#E8BA3C` / `#3D2E0A` |
| `--surface-page` | `#F8F8F8` | `#141414` |
| `--surface-card` | `#FFFFFF` | `#1E1E1E` |
| `--surface-elevated` | `#FFFFFF` | `#2A2A2A` |
| `--surface-sidebar` | `linear-gradient(180deg, #072D3B, #0A3D4F)` | `linear-gradient(180deg, #0A0F14, #0F1A22)` |
| `--surface-header` | `linear-gradient(160deg, #072D3B, #0E4F65, #1A7A9A)` | `linear-gradient(160deg, #0A0F14, #0F1A22, #13384A)` |

All text/background combinations maintain WCAG AA: 4.5:1 normal text, 3:1 large text.

---

## Typography

### Font Stack

- **Headings:** `'Manrope', 'Segoe UI', system-ui, sans-serif`
- **Body:** `'Inter', 'Segoe UI', system-ui, sans-serif`
- **Monospace:** `'JetBrains Mono', 'Cascadia Code', 'Consolas', monospace`

### Type Scale (1.25 ratio — Major Third)

| Token | Size | Weight | Line Height | Font | Usage |
|---|---|---|---|---|---|
| `--text-display` | 48px / 3rem | 800 | 1.1 | Manrope | Marketing hero, onboarding |
| `--text-h1` | 36px / 2.25rem | 700 | 1.2 | Manrope | Page titles |
| `--text-h2` | 28px / 1.75rem | 700 | 1.25 | Manrope | Section headings |
| `--text-h3` | 22px / 1.375rem | 600 | 1.3 | Manrope | Card titles, subsections |
| `--text-h4` | 18px / 1.125rem | 600 | 1.35 | Manrope | Minor headings |
| `--text-body-lg` | 18px / 1.125rem | 400 | 1.6 | Inter | Conversational cards, lead text |
| `--text-body` | 16px / 1rem | 400 | 1.6 | Inter | Default body |
| `--text-body-sm` | 14px / 0.875rem | 400 | 1.5 | Inter | Secondary text, table cells |
| `--text-caption` | 12px / 0.75rem | 500 | 1.4 | Inter | Labels, timestamps, metadata |
| `--text-overline` | 11px / 0.6875rem | 600 | 1.4 | Inter | Section labels (uppercase, `letter-spacing: 0.08em`) |

### Conversational Text Styling

- Amounts: Manrope 700, `--color-primary-600`
- Date highlights: Inter 600, `--color-secondary-500`
- Entity names: Inter 600, `--color-neutral-900`
- Status language: Semantic color, weight 600

### App Divergence

- **Management:** Primarily `body-sm` through `h3`. Denser. `body-lg` only for conversational summary card.
- **Member:** `body` and `body-lg` used liberally. Content breathes. `h1` for greeting.

---

## Spacing & Layout

### Spacing Scale (8px base)

| Token | Value | Usage |
|---|---|---|
| `--space-1` | 4px | Icon-to-label gaps |
| `--space-2` | 8px | Compact padding, related item gaps |
| `--space-3` | 12px | Small component padding |
| `--space-4` | 16px | Default card padding (management), form gaps |
| `--space-5` | 20px | Medium gaps |
| `--space-6` | 24px | Card padding (member app), section gaps |
| `--space-8` | 32px | Large section gaps |
| `--space-10` | 40px | Section separation |
| `--space-12` | 48px | Major section separation |
| `--space-16` | 64px | Page-level padding |
| `--space-20` | 80px | Large page sections |

### Border Radius

| Token | Value | Usage |
|---|---|---|
| `--radius-sm` | 4px | Small badges, tags |
| `--radius-md` | 8px | **Default** — buttons, inputs, cards |
| `--radius-lg` | 12px | Larger cards, modals, feed items |
| `--radius-full` | 9999px | Avatars, pills |

### Breakpoints

| Token | Value |
|---|---|
| `--bp-sm` | 640px |
| `--bp-md` | 768px |
| `--bp-lg` | 1024px |
| `--bp-xl` | 1280px |
| `--bp-2xl` | 1536px |

### Grid

- **Management:** 12-column, `gap: var(--space-4)`, sidebar 230px fixed
- **Member:** Single column, max-width 428px, `padding: 0 var(--space-4)`
- **Container max-width:** 1280px

### Density Divergence

- **Management:** `space-2` to `space-4` between rows, compact stat cards
- **Member:** `space-4` to `space-6` between feed items, generous card padding

---

## Elevation & Shadows

### Light Mode

| Token | Value | Usage |
|---|---|---|
| `--shadow-xs` | `0 1px 2px rgba(26,26,26,0.05)` | Buttons, inputs |
| `--shadow-sm` | `0 1px 3px rgba(26,26,26,0.08), 0 1px 2px rgba(26,26,26,0.04)` | Cards at rest |
| `--shadow-md` | `0 4px 6px rgba(26,26,26,0.07), 0 2px 4px rgba(26,26,26,0.04)` | Cards hover |
| `--shadow-lg` | `0 10px 15px rgba(26,26,26,0.08), 0 4px 6px rgba(26,26,26,0.04)` | Dropdowns |
| `--shadow-xl` | `0 20px 25px rgba(26,26,26,0.08), 0 8px 10px rgba(26,26,26,0.03)` | Modals |
| `--shadow-2xl` | `0 25px 50px rgba(26,26,26,0.15)` | Toasts |
| `--shadow-inner` | `inset 0 2px 4px rgba(26,26,26,0.05)` | Pressed states |

### Dark Mode

| Token | Value |
|---|---|
| `--shadow-xs` | `0 1px 2px rgba(0,0,0,0.2)` |
| `--shadow-sm` | `0 1px 3px rgba(0,0,0,0.25), 0 1px 2px rgba(0,0,0,0.15)` |
| `--shadow-md` | `0 4px 6px rgba(0,0,0,0.25), 0 2px 4px rgba(0,0,0,0.15)` |
| `--shadow-lg` | `0 10px 15px rgba(0,0,0,0.3), 0 4px 6px rgba(0,0,0,0.15)` |
| `--shadow-xl` | `0 20px 25px rgba(0,0,0,0.3), 0 8px 10px rgba(0,0,0,0.12)` |
| `--shadow-2xl` | `0 25px 50px rgba(0,0,0,0.4)` |
| `--shadow-inner` | `inset 0 2px 4px rgba(0,0,0,0.2)` |

### Elevation Hierarchy

| Level | Shadow | Examples |
|---|---|---|
| 0 — Flat | none | Page background |
| 1 — Resting | `shadow-sm` | Cards, stat tiles, feed items |
| 2 — Raised | `shadow-md` | Hovered cards |
| 3 — Floating | `shadow-lg` | Dropdowns, popovers |
| 4 — Overlay | `shadow-xl` | Modals, drawers |
| 5 — Top | `shadow-2xl` | Toasts |

### Interactive Depth

- Cards: rest level 1 → hover level 2 + `translateY(-2px)` over `200ms ease`
- Conversational card: rest level 1 → hover level 2 + `translateY(-3px)` (extra lift for hero element)
- Buttons: rest xs → active `shadow-inner`
- Modals: backdrop `rgba(26,26,26,0.5)` light / `rgba(0,0,0,0.7)` dark

---

## Motion & Interaction

### Easing Curves

| Token | Value | Usage |
|---|---|---|
| `--ease-default` | `cubic-bezier(0.25, 0.1, 0.25, 1)` | General transitions |
| `--ease-out` | `cubic-bezier(0.0, 0.0, 0.2, 1)` | Elements entering view |
| `--ease-in` | `cubic-bezier(0.4, 0.0, 1, 1)` | Elements leaving view |
| `--ease-spring` | `cubic-bezier(0.34, 1.56, 0.64, 1)` | Playful interactions (member app only) |
| `--ease-smooth` | `cubic-bezier(0.4, 0.0, 0.2, 1)` | Page transitions |

### Durations

| Token | Value | Usage |
|---|---|---|
| `--duration-instant` | `100ms` | Button color, icon swap |
| `--duration-fast` | `150ms` | Hover states, focus rings |
| `--duration-normal` | `200ms` | Card lift, shadow transitions |
| `--duration-moderate` | `300ms` | Panel expand/collapse |
| `--duration-slow` | `400ms` | Page entrances, modal open |
| `--duration-stagger` | `60ms` | Delay between sequential items |

### Signature Interaction Patterns

**1. Staggered Reveal**
List items entering the viewport fade in with stagger delay:

- Each item: `opacity 0→1`, transform to origin, `400ms ease-out`
- Stagger: `60ms` between items
- Management: `translateX(-6px)→0` (data flows left-to-right)
- Member: `translateY(8px)→0` (content rises up)

**2. Card Depth Response**

- Rest: `shadow-sm`, `translateY(0)`
- Hover: `shadow-md`, `translateY(-2px)`, `200ms ease-default`
- Conversational card (hero): `translateY(-3px)` on hover

**3. Progress & Status Animations**

- Progress bar fill: `0→value` over `800ms ease-out` on first appearance
- Live pulse dot: `opacity 1→0.4→1` over `2s ease-in-out infinite`
- Status transitions: cross-fade `300ms`

**4. Button Micro-interactions**

- Hover: `scale(1.02)`, `duration-fast`, `ease-default`
- Active: `scale(0.98)`, `shadow-inner`, `duration-instant`
- Focus: `0 0 0 3px primary-300`, `duration-fast`
- Member "Pay Now": uses `ease-spring` on hover
- Management buttons: `ease-default` only

**5. Page Transitions**

- Outgoing: `opacity 1→0`, `translateY(-4px)`, `200ms ease-in`
- Incoming: `opacity 0→1`, `translateY(4px)`, `300ms ease-out`, `100ms delay`

**6. Conversational Card Entrance**

- `translateY(16px)→0`, `opacity 0→1`, `500ms ease-out`
- Slower and larger motion — hero element announces itself

### prefers-reduced-motion

All animations collapse to `100ms` with no transforms. Stagger delays become 0. Pulse dot becomes static. Fully functional, no motion personality.

### App Divergence

- **Management:** `ease-default` and `ease-smooth` only. No spring. Durations lean fast/normal.
- **Member:** Adds `ease-spring` for primary CTAs. Durations lean normal/moderate. More generous stagger.

---

## Components

### Buttons

**Variants:**

| Variant | Style | Usage |
|---|---|---|
| Primary | `background: linear-gradient(135deg, primary-600, primary-700)`, white text, `shadow-xs` | Main CTAs |
| Secondary | `border: 1.5px solid primary-600`, primary-600 text, transparent bg | Secondary actions |
| Ghost | No border, primary-600 text | Tertiary, in-table links |
| Destructive | `background: error-600`, white text | Delete, reject |

**Sizes:**

| Size | Padding | Font Size | Min Height |
|---|---|---|---|
| Small | `6px 12px` | 13px | 32px |
| Medium | `10px 20px` | 14px | 40px |
| Large | `14px 28px` | 16px | 48px |

**States:** hover (scale 1.02, darken), active (scale 0.98, shadow-inner), focus-visible (3px ring primary-300), disabled (opacity 0.5, no pointer events).

**App divergence:** Member "Pay Now" uses gradient + elevated shadow. Management uses flat primary.

### Cards

| Type | Description |
|---|---|
| Default | White bg, `radius-md`, `shadow-sm`, `padding: space-6`. Hover lifts. |
| Stat | Default + 3px colored top border. Management only. |
| Conversational | Larger padding, `body-lg` text. Status summaries. |
| Feed item | `radius-lg`, `shadow-sm`, icon/avatar + text. Stagger entrance. |
| Interactive | Default + pointer cursor, hover lifts. Clickable items. |

### Form Inputs

| Property | Value |
|---|---|
| Height | 44px |
| Border | `1.5px solid neutral-400` |
| Radius | `radius-md` |
| Font | Inter 400, 14px |
| Focus | Border `primary-600`, ring `0 0 0 3px primary-200` |
| Error | Border `error-600`, ring `rgba(192,57,43,0.15)` |
| Disabled | `background: neutral-100`, `opacity: 0.6` |

Variants: text, textarea (min-height 100px), select, checkbox (16px, 4px radius), radio (16px, round), toggle (44×24px).

### Badges & Pills

| Type | Style | Usage |
|---|---|---|
| Status pill | `radius-full`, 10px, 600 weight, semantic bg-100/text-600 | Current, Past Due, Delinquent |
| Category tag | `radius-sm`, 9px, 600 weight, tinted bg | Amenity, Meeting, Notice |
| Count badge | `radius-full`, 9px, white on `secondary-500` | Sidebar counts |

### Alerts

4 semantic types. All: `radius-md`, `padding: space-4`, 4px left border, semantic-100 bg, icon + message + optional action.

Dark mode: backgrounds shift to semantic-600 at 10% opacity.

### Tables (Management)

| Element | Style |
|---|---|
| Header | `neutral-50` bg, 10px uppercase Inter 600, `neutral-500` |
| Row | Alternating `white` / `neutral-50`, `border-bottom: 1px solid neutral-200` |
| Row hover | `primary-50` |
| Cell padding | `12px 16px` |
| Avatar cell | 28px avatar + name |
| Amount | JetBrains Mono, negative `error-600`, zero `success-600` |

### Navigation

**Management sidebar:** 230px, `surface-sidebar`, 34px logo mark, section labels 9px uppercase, items 12.5px, active `rgba(26,122,154,0.3)`.

**Member bottom tab bar:** 56px, 5 items max, icon 18px + label 9px, active `primary-600`.

**Breadcrumbs:** Inter 11px, `neutral-500`, links `primary-600`.

**Tabs:** Inter 13px 500, active `primary-600` + 2px bottom border.

### Modals

| Property | Value |
|---|---|
| Backdrop | `rgba(26,26,26,0.5)` / `rgba(0,0,0,0.7)` |
| Container | `surface-elevated`, `radius-lg`, `shadow-xl`, max-width 480px |
| Padding | `space-6` |
| Entry | `opacity 0→1 + translateY(8px)→0`, `400ms ease-out` |

### Activity Stream / Community Pulse

The signature component defining Quorant's personality.

| Element | Management | Member |
|---|---|---|
| Header | "Recent Activity" + live dot + filter tabs | "Community Pulse" + live dot |
| Avatar | 32px, initials, role-colored | 30px, emoji/icon, gradient |
| Text | Factual: "Sarah Chen paid $285" | Conversational: "The community pool opens May 1st" |
| Meta | Unit + timestamp | Timestamp + category tag |
| Action | "Review >" link | Tap whole card |
| Entrance | Stagger `translateX(-6px)` | Stagger `translateY(8px)` |

---

## Conversational UI Guidelines

**Principle:** Status should feel like a helpful neighbor, not an accounting system.

### Patterns

| Context | Member App | Management Portal |
|---|---|---|
| Balance due | "Your April assessment of $285.00 is ready. Autopay handles it on the 12th." | "Oakridge Commons is in good shape this month." |
| Current status | "You're all set — no action needed." | "Collections at 94.2% — up from last month." |
| Past due | "Your March assessment is past due. A $25 late fee applies after the 20th." | "3 accounts need attention before they go delinquent." |
| Payment received | "Thanks — your $285.00 payment posted today." | "Sarah Chen paid April assessment — $285.00" |
| Work order update | "Your parking lot light request is now in progress." | "Priya Patel reported a streetlight outage — WO #247 created." |

### Rules

1. Lead with what matters to the user, not the system state.
2. Name dates naturally: "next Tuesday" or "the 12th" — not ISO dates.
3. Use names in management, "you/your" in member.
4. Escalation language scales with severity — calm → helpful nudge → direct (never threatening).
5. Management activity stream is third-person narrative.
6. Member community pulse is announcement-style.

---

## Layout Patterns

### Management Portal

| Layout | Structure |
|---|---|
| Dashboard | Conversational summary → stat grid (4-col) → activity stream + mini charts (2-col) |
| List view | Filter bar → sortable data table → pagination |
| Detail view | Entity header + status → tabbed content → action sidebar |
| Form view | Stepped or single-page form → validation → confirmation |

### Member App

| Layout | Structure |
|---|---|
| Home | Header greeting → conversational card → quick actions (4-col) → community pulse → upcoming |
| Payment flow | Balance summary → method selection → amount confirm → success |
| Request flow | Category select → form fields → photo upload → confirmation |
| Detail view | Status header → update timeline → action buttons |

### Empty States

**Member (conversational):**
- No payments: "You're all caught up — no payments due right now."
- No violations: "Clean record — no violations on file."
- No activity: "It's quiet in the neighborhood today."

**Management (functional):**
- No residents: "No residents added yet. Import a CSV or add them manually."

---

## Accessibility

- All text/background combinations meet WCAG AA (4.5:1 normal, 3:1 large)
- Visible `:focus-visible` outlines on all interactive elements (3px ring)
- `prefers-reduced-motion` disables all animations and transitions
- Color is never the sole indicator — always paired with text or icons
- Minimum touch target: 44×44px
- Minimum tap spacing: 8px between adjacent interactive elements
- Body font minimum: 16px (prevents iOS auto-zoom)

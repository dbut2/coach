# Naomi — Design System

*A design-led, library-agnostic specification for Naomi's interface. The tokens
and component definitions here are the **source of truth**; they are intended to
be realised identically in Figma (as variables + components) and in code (as CSS
custom properties + hand-built components). No component library — DaisyUI,
Bootstrap, or otherwise — is part of this system. Tailwind, if used, is permitted
only as a low-level utility/primitive engine, never as a source of component
appearance.*

> **Why no library.** A component library ships its own opinions — button heights,
> radii, hover states, the very idea of what "primary" looks like. Theming it only
> tints those opinions; the ceiling stays "nice generic." This system defines the
> component vocabulary itself, so the result is *designed*, not inherited.

Rendered references (the earlier DaisyUI-based pass, kept only for layout
intent): [`docs/screenshots/`](screenshots). Token source once implemented:
CSS custom properties; current interim build: [`frontend/input.css`](../frontend/input.css).

---

## 1. Product principle: one screen, the chat

Naomi is a coach you text, not a dashboard. The whole app is a **single
conversational thread**, opening to the **latest message**. There is exactly one
justified non-chat screen — the **full plan** (the week/block ahead). Everything
else is **text first**, with **cards** only where a structured shape (a workout, a
set of numbers, a proposed change) is genuinely clearer than a sentence.

- **Text preferred, cards where relevant** — cards enhance the coach's words,
  never replace them.
- **Numbers on request** — pace, load, zones appear as a stat card when asked,
  never front-and-center.
- **Phone-first** — a single mobile column (max 28rem). Web is a later target;
  don't hard-block it, but design for the phone.

## 2. Voice & behaviour

**Warm training partner.** In-your-corner, upbeat, plain-spoken — a mate who knows
the training science. Never hypey, never clinical. Every judgement is grounded in
measured data and says what it rests on (measurements are facts; projections are
the coach's call — see the brief's Amendment A).

**Proactive but human.** Initiates check-ins, post-run debriefs, the occasional
"how's the knee?", and backs off naturally on weak engagement.

**Onboarding is conversational.** No wizard. The coach talks the runner through
goals and history, and *narrates* background work ("pulling your last few months
while we talk").

## 3. Brand

| Pillar | Decision |
|---|---|
| **Name** | **Naomi** — warm, human first name; reinforces "texting a person." Default for `COACH_NAME`. |
| **Personality** | Warm training partner (§2). |
| **Look** | **Volt** — electric lime on near-black. Energetic, athletic, deliberately unlike Strava's orange. |
| **Avatar** | Abstract human (head + shoulders) in ink on a Volt disc with a soft glow. Reads as "a person," never a mascot. App icon, login hero, chat avatar. |

---

## 4. Foundations (tokens)

Tokens are the contract. Names below are the canonical identifiers; map them
1:1 to Figma variables (grouped by the collection in the first column) and to CSS
custom properties (`--<group>-<name>`, e.g. `--color-ink-900`).

### 4.1 Color — primitives

Two ramps and a small semantic set. Primitives are raw values; **components must
reference semantic roles (§4.2), not primitives directly.**

**Ink (neutral ramp)** — warm-neutral darks through a warm white.

| Token | Hex |
|---|---|
| `ink-950` | `#09090B` |
| `ink-900` | `#121317` |
| `ink-850` | `#16181D` |
| `ink-800` | `#1C1E24` |
| `ink-750` | `#23262D` |
| `ink-700` | `#2C2F38` |
| `ink-500` | `#4B505C` |
| `ink-400` | `#6B7180` |
| `ink-300` | `#9298A6` |
| `ink-200` | `#BFC4CF` |
| `ink-50` | `#F3F4F1` |

**Volt (accent ramp)**

| Token | Hex | Note |
|---|---|---|
| `volt-300` | `#E6FF59` | light / hover |
| `volt-500` | `#D4F500` | core accent |
| `volt-700` | `#A9C400` | pressed |
| `volt-tint` | `rgba(212,245,0,0.12)` | accent-on-dark fills |
| `volt-glow` | `rgba(212,245,0,0.45)` | CTA glow |

**Signal colors** (kept distinct from the accent so "good" never reads as "button").

| Token | Hex | Meaning |
|---|---|---|
| `green-500` | `#5FDD8A` | positive: connected, synced, on-track |
| `amber-500` | `#F5B83D` | caution: debug/tool traces |
| `rose-500` | `#FB6F84` | critical: destructive actions |
| `cyan-500` | `#58C7E6` | informational |

**Partner colors** (immutable): Strava `#FC4C02`, Garmin `#007CC3`.

### 4.2 Color — semantic roles

Components reference these. Each maps to a primitive (dark theme is the only theme
for now; a light theme would remap the same roles).

| Role | → primitive | Use |
|---|---|---|
| `bg/app` | `ink-950` | behind the panel |
| `bg/surface` | `ink-900` | the panel / page |
| `bg/raised` | `ink-800` | coach bubbles, cards |
| `bg/sunken` | `ink-850` | insets inside cards (rationale chip) |
| `border/hairline` | white @ 6% | default dividers |
| `border/strong` | white @ 12% | card edges, inputs |
| `text/primary` | `ink-50` | body and headings |
| `text/secondary` | `ink-300` | supporting copy |
| `text/muted` | `ink-400` | timestamps, hints |
| `accent` | `volt-500` | primary actions, runner bubble, emphasis |
| `on-accent` | `ink-950` | text/glyphs on accent |
| `positive` / `caution` / `critical` / `info` | `green/amber/rose/cyan-500` | signals |

Contrast: `text/primary` on `bg/surface` and `on-accent` on `accent` both exceed
WCAG AA. `text/muted` is reserved for non-essential meta only.

### 4.3 Typography

Two families, self-hosted. **Space Grotesk** (display) for identity, titles, and
numerals; **Inter** (text) for everything read in sentences.

Type ramp (name · font/size/weight/line-height/tracking · use):

| Token | Spec | Use |
|---|---|---|
| `display` | Space Grotesk · 34 / 700 / 1.05 / −0.02em | login wordmark |
| `title-lg` | Space Grotesk · 22 / 700 / 1.15 / −0.02em | empty-state, hero |
| `title` | Space Grotesk · 17 / 700 / 1.2 / −0.01em | app bar, card titles, coach name |
| `numeric` | Space Grotesk · 20 / 700 / 1.0 / −0.01em · tabular | stat values, distance badges |
| `body` | Inter · 15 / 440 / 1.5 / 0 | message text, paragraphs |
| `body-strong` | Inter · 15 / 600 / 1.5 | runner message, emphasis |
| `label` | Inter · 13 / 500 / 1.3 | inputs, secondary detail |
| `caption` | Inter · 12 / 500 / 1.4 | hints, fine print |
| `overline` | Inter · 11 / 600 / 1.0 / 0.06em · UPPERCASE | section + card labels |

(Inter weight `440` = Inter at 400 with a touch more presence on dark; use 400 if
a variable axis isn't available.)

### 4.4 Space

4px base grid. Use only these steps.

`space-0` 0 · `space-1` 4 · `space-2` 8 · `space-3` 12 · `space-4` 16 ·
`space-5` 20 · `space-6` 24 · `space-8` 32 · `space-10` 40 · `space-12` 48.

Rhythm: in-bubble padding `space-3`/`space-4`; gap between turns `space-3`;
screen gutter `space-4`; section gap `space-6`.

### 4.5 Radius

`radius-sm` 8 · `radius-md` 12 · `radius-lg` 16 · `radius-xl` 20 · `radius-pill`
999. **Bubble** uses `radius-lg` with the sender-side corner collapsed to
`radius-sm` (coach: top-left; runner: bottom-right).

### 4.6 Elevation

A flat dark UI; depth comes from hairlines and one accent glow, not heavy shadows.

| Token | Value | Use |
|---|---|---|
| `elev-flat` | border `border/hairline`, no shadow | dividers, list rows |
| `elev-card` | border `border/strong` + `0 1px 2px rgba(0,0,0,.40)` | cards, bubbles |
| `elev-bar` | `0 1px 0 rgba(0,0,0,.5)` + 12px backdrop blur | app bar, composer |
| `glow-accent` | `0 2px 16px -4px volt-glow` | primary CTA, avatar |

### 4.7 Motion

| Token | Value |
|---|---|
| `dur-instant` | 80ms |
| `dur-fast` | 140ms |
| `dur-base` | 220ms |
| `dur-slow` | 360ms |
| `ease-out` | `cubic-bezier(0.2, 0, 0, 1)` (enter / move) |
| `ease-in` | `cubic-bezier(0.4, 0, 1, 1)` (exit) |

Named motions: **bubble-in** (translateY 8→0 + fade, `dur-base ease-out`),
**seen-tick** (scale 0.6→1 on the ✓✓, `dur-fast`), **typing-dots** (3-dot
staggered opacity loop), **avatar-pulse** (Volt glow breathes, ~3s, the ambient
"alive" cue). Respect `prefers-reduced-motion`: drop translate/scale, keep fades.

### 4.8 Iconography & layout

Icons: 1.75px stroke, rounded caps/joins, sizes 16 / 18 / 20. Source: Lucide
(self-hosted) or an equivalent outline set at matching weight. Layout: single
column, `max-width 28rem`, gutter `space-4`, honor `safe-area-inset-*`.

---

## 5. Components

Each is defined from scratch: anatomy → sizing → states → tokens. No library
classes. "States" lists every visual state the component must define.

### 5.1 Button

- **Variants:** `primary` (accent fill, `on-accent` text, `glow-accent`),
  `secondary` (`bg/raised` fill, `border/strong`, `text/primary`), `ghost`
  (transparent, `text/secondary`), `destructive` (ghost with `critical` text).
- **Sizes:** `sm` h-36 px-12 `label`; `md` h-44 px-16 `body-strong`; `block`
  full-width md.
- **Shape:** `radius-pill`.
- **States:** default · hover (primary→`volt-300`; others→raise bg one step) ·
  active (primary→`volt-700`, glow off) · focus-visible (2px `accent` ring, 2px
  offset) · disabled (40% opacity, no glow) · loading (spinner replaces label,
  width held).

### 5.2 Text field & textarea

- **Anatomy:** `overline`/`label` caption · field · optional helper/error.
- **Field:** `bg/surface`, `border/strong`, `radius-md`, h-44 (textarea
  min-h-44, auto-grow to 128), padding `space-3`, `body` text, `text/muted`
  placeholder.
- **States:** default · focus (border→`accent`, 3px `volt-tint` ring) · error
  (border→`critical`, helper in `critical`) · disabled (sunken, 50%).

### 5.3 Composer (message input)

A pill containing an auto-grow textarea + send button. `bg/raised`,
`radius-pill`, `border/strong`; on focus-within the whole pill gets the field
focus ring. Send = `primary` button `sm` circular (36), arrow-up glyph; disabled
until non-empty. Docked in the footer with `elev-bar` + safe-area padding.

### 5.4 Coach avatar

Volt disc, `on-accent` abstract-human glyph (head circle + shoulders arc),
`glow-accent`. Sizes: 32 (chat), 36 (app bar), 96 (login). `avatar-pulse` only
when the coach is actively working.

### 5.5 Message bubble

- **Coach:** left, beside a 32 avatar; `bg/raised`, `radius-lg` w/ top-left
  `radius-sm`; `body` text; max-width 82%. Optional cards (§5.6–5.7) stack below
  the text inside the column; `caption` timestamp in `text/muted` beneath.
- **Runner:** right; `accent` fill, `on-accent` `body-strong`; `radius-lg` w/
  bottom-right `radius-sm`; max-width 82%. Below the last runner bubble: a
  delivery row — `caption` timestamp + **✓✓ Seen** (accent ✓✓) once read.
- **Enter:** bubble-in.

### 5.6 Workout card

Inset panel (`bg/sunken`, `border/hairline`, `radius-md`, padding `space-3`):
`overline` when-label in `accent` · `numeric` distance badge (pill, `bg/surface`)
· `title` name · `caption` detail in `text/secondary` · optional footer row
(hairline-topped) "Synced to your watch" with a watch glyph in `positive`.

### 5.7 Stat strip

Row of 3 equal cells (`bg/sunken`, `border/hairline`, `radius-md`, centered):
`numeric` value · `overline`/`caption` label · optional `caption` hint in
`text/muted`. Used for numbers-on-request.

### 5.8 Proposal card

`bg/raised`, `border/strong`, `radius-lg`, padding `space-4`: header row
(`overline` weekday+date in `accent` · `numeric` distance badge) · `title`
workout · `caption` detail · the coach's **one-line rationale** in a `bg/sunken`
chip with a speech glyph · actions row: `destructive` Reject + `primary` Approve,
equal width. Empty state: centered `positive`/muted check, `title-lg` + `caption`.

### 5.9 List row (connections, settings)

48-min row: leading 40 brand tile (brand color, `radius-md`, white glyph) ·
title (`label`/`body-strong`) + status (`caption`, with a `positive` dot when
connected) · trailing action(s). Rows separated by `border/hairline` inside a
`bg/raised` `radius-lg` group.

### 5.10 App bar

56-tall, `elev-bar`. Leading: back chevron (sub-screens) or 36 avatar + name +
status (chat). Trailing: icon buttons (ghost). Title in `title`.

### 5.11 Banner (inline)

Accent-tinted bar (`volt-tint` bg, `accent`/30 border, `radius-md`): leading
glyph in `accent` · `body-strong` message · trailing chevron. Used for "N plan
changes to approve".

### 5.12 Status & indicators

- **Status dot:** 6px, `positive` (online/connected).
- **Delivered → Seen:** runner message shows *delivered*, then **Seen** (✓✓
  `accent`, seen-tick) once the coach starts working.
- **Typing dots:** coach-side, in a bubble, **only while message text is actually
  streaming** — never for tool calls or background work.
- **Day divider:** centered `overline` chip on `bg/raised`, `radius-pill`.

---

## 6. Screens

Composed from §5 only.

- **Login** — centered glowing avatar → `display` "Naomi" → tagline → value line;
  bottom-docked Strava button (partner color) + `caption` permissions note; faint
  Volt radial glow from the top.
- **Chat (home)** — app bar (avatar + name + status, debug + settings) · optional
  banner · thread (day divider, bubbles, cards, Seen) · composer.
- **Plan changes** — back app bar · intro line · proposal cards · empty state.
- **Settings** — sections (`overline` headers): Profile (text field + Save),
  Connections (list rows; Garmin shows its email/password caveat inline);
  destructive actions in `critical`; sign out at bottom.
- **Full plan (to design)** — the one non-chat screen: the week/block ahead.
  Per-day rows (rest / easy / workout / long) with distance + one-line focus;
  today marked; completed vs upcoming distinguished; race day as the anchor.
  Read-only — edits happen by talking to Naomi.

## 7. Interaction model

- **Acknowledgement, not fake typing** (§5.12): delivered → Seen; typing dots
  only on streaming text.
- **Auto-scroll** to newest; **auto-grow** composer to 128.
- **Proactivity cues** kept quiet — avatar pulse, a new coach bubble; no nags.
- **Reduced motion** honored throughout (§4.7).

## 8. Out of scope / deferred

History search (Q&A covers recall; defer), quick-reply chips (no), in-app Garmin
auth changes (no). Debug/tool-trace view is a **dev tool**, gated behind a feature
flag (LaunchDarkly / Statsig / Flagsmith / Unleash), not shipped to runners.
Per-change plan approval is a **short-term trust step** toward autonomous plan
adaptation.

## 9. Implementation guidance (no library)

- **Tokens → CSS custom properties** on `:root` (one variable per §4 token).
  Optionally drive Tailwind via `@theme` so utilities resolve to these vars, but
  do **not** reintroduce DaisyUI (`@plugin "daisyui"`) or any component library.
- **Components → a hand-authored `components.css` layer** (or small templ
  partials with explicit utility classes). Each component in §5 becomes one class
  / partial with its states defined explicitly — no inherited button/input/card.
- **Migration from the interim build:** the current `frontend/input.css` themes
  DaisyUI; replace its `@plugin "daisyui"` blocks with the token `:root` block and
  the components layer, then strip DaisyUI classes (`btn`, `input`, `card`,
  `navbar`, `loading`, …) from `go/web/*.templ` in favour of the new classes.
- **Figma:** create variable collections matching §4 (Color/primitive,
  Color/semantic, Space, Radius, Type, Elevation, Motion), build the §5 components
  as Figma components with variants for each state, then compose §6 screens from
  instances. The token names are intentionally identical across Figma and code so
  the two stay in lockstep.

## 10. Figma build prompt

Paste into a desktop Claude Code session on this repo (MCP write approvals work
there):

> Build Naomi's design system in a new Figma file from `docs/design.md`. First
> create variable collections for every token in §4 (exact names + values). Then
> build the §5 components as Figma components with a variant per state. Then
> compose the §6 screens (Login, Chat, Plan changes, Settings, Full plan) at
> 390×844 from those component instances only. Use Space Grotesk + Inter. Do not
> use any imported UI kit — this is a bespoke system.

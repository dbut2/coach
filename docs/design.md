# Naomi — UX & Design System

*The product's interface principles, brand identity, and a self-contained spec
for building the screens in Figma or any other tool. This document travels with
the project; the compiled source of truth for tokens lives in
[`frontend/input.css`](../frontend/input.css), and the screens live in
[`go/web/*.templ`](../go/web).*

Rendered references: [`docs/screenshots/`](screenshots) (login, conversation,
settings, proposals).

---

## 1. Product principle: one screen, the chat

Naomi is a coach you text, not a dashboard. The entire app is a **single
conversational thread**. There is exactly one justified non-chat screen — the
**full plan** (the week/block ahead). Everything else the coach communicates is
**text first**, with **cards** only where a structured shape (a workout, a set of
numbers, a proposed change) is genuinely clearer than a sentence.

Derived rules:

- **Opens to the latest message.** No home screen, no feed, no landing.
- **Text preferred, cards where relevant.** Cards are an enhancement beneath the
  coach's words, never a replacement for them.
- **Numbers on request.** Pace, load, and zones are available when the runner
  asks — surfaced as a stat card — but are never front-and-center.
- **Phone-first.** Layout is capped at a single mobile column (`max-w-md`). Web
  is a later target; nothing should hard-block it, but design for the phone.

## 2. Voice & behaviour

**Voice: a warm training partner.** Friendly, in-your-corner, upbeat — talks like
a mate who happens to know the training science. Never hypey, never clinical.
Grounded in real measured data (every judgement says what it rests on; see the
brief's Amendment A — measurements are facts, projections are the coach's call).

**Proactivity: human, not pushy.** Naomi initiates — morning check-ins, post-run
debriefs, the occasional "how's the knee?" — and naturally backs off when
engagement is weak, the way a real coach reads the room.

**Onboarding is conversational.** No setup wizard. After the Strava connect, the
coach talks the runner through goals and history. While Strava back-loads 90 days
of activity, Naomi *says so* ("I'm pulling your last few months in the
background — while that loads, tell me about your next race") and uses the wait.

## 3. Brand identity

| Pillar | Decision |
|---|---|
| **Name** | **Naomi** — a warm, human first name. Reinforces "texting a person." Configurable via `COACH_NAME`; Naomi is the default. |
| **Personality** | Warm training partner (see §2). |
| **Look** | **Volt** — electric lime on near-black. Energetic, athletic, deliberately distinct from Strava's orange. |
| **Avatar** | An **abstract human** mark (head + shoulders) in dark on a Volt disc with a soft glow. Reads as "a person," never a stock mascot. Reused as app icon, login hero, and the coach's chat avatar. |

## 4. Design tokens

All values are the live theme in `frontend/input.css` (DaisyUI v5 `naomi` theme).

### Color

| Token | Hex | Use |
|---|---|---|
| `base-300` | `#0a0a0b` | App background behind the panel |
| `base-100` | `#131417` | Chat panel / page surface |
| `base-200` | `#1c1e23` | Coach bubbles, raised cards |
| `base-content` | `#f3f4f1` | Primary text (warm white) |
| `primary` (Volt) | `#d4f500` | User bubbles, primary buttons, accents, focus ring |
| `primary-content` | `#0a0a0b` | Text/glyphs on Volt |
| `success` | `#9ae600` | "Connected", synced-to-watch, status dot |
| `warning` | `#f6c945` | Debug/tool traces |
| `error` | `#fb7185` | Destructive actions (reject, disconnect, sign out) |
| `info` | `#56c8e6` | Informational accents |

Brand partners keep their own colors: **Strava** `#FC4C02`, **Garmin** `#007CC3`.

Borders/dividers: white at 5–10% opacity (`border-white/5`…`/10`) over the dark
surfaces. Volt buttons carry a soft glow: `0 2px 10px -2px rgba(212,245,0,0.5)`.

### Radius

`box` 1.25rem (cards), `field` 0.75rem (inputs, small cards), `selector` 1rem.
Bubbles are `rounded-2xl` with one squared corner toward the sender
(coach: top-left; runner: bottom-right). Composer is a `rounded-3xl` pill.

### Type

- **Display** — Space Grotesk (500/700), `letter-spacing: -0.02em`. Coach name,
  screen titles, card titles, stat values, login wordmark.
- **Body** — Inter (400/500/600). Message text, labels, detail.

| Role | Size / weight |
|---|---|
| Login wordmark | 36px / 700 display |
| Screen / coach-name title | 15–16px / 700 display |
| Message text | 15px / 400–500 body |
| Card title | 16px / 700 display |
| Stat value | 18px / 700 display |
| Section label, meta | 11px / 600 body, uppercase + tracked for section labels |

Fonts are self-hosted (`go/web/assets/fonts/`); no Google Fonts dependency.

## 5. Components

- **Coach avatar** — Volt disc, dark abstract-human glyph (head circle +
  shoulders arc), soft Volt glow. Sizes: 24 (login hero is 96), 32 (chat), 36
  (header).
- **Coach bubble** — `base-200`, `rounded-2xl rounded-tl-md`, 15px text; optional
  card(s) and a muted timestamp beneath; left-aligned beside the avatar.
- **Runner bubble** — `primary` (Volt) fill, `primary-content` text,
  `rounded-2xl rounded-br-md`, right-aligned. Below the last one: timestamp + a
  Volt **✓✓ Seen** when the coach has read it.
- **Workout card** — bordered `base-300/60` panel: Volt uppercase when-label, a
  distance badge, Space Grotesk name, muted detail, and a `success` "Synced to
  your watch" row when pushed.
- **Stat strip** — 3-up grid of bordered cells, each a big Space Grotesk value, a
  label, and a small hint. Used for numbers-on-request.
- **Composer** — pill with auto-growing textarea and a Volt circular send button;
  Volt focus ring on focus-within.
- **Pending banner** — Volt-tinted bar above the thread linking to plan changes.
- **Section header** — 11px uppercase tracked label (settings, lists).

## 6. Screen specs

**Login.** Centered: glowing Volt avatar → "Naomi" wordmark → "Your AI running
coach" → one-line value prop. Pinned bottom: full-width Strava-orange "Continue
with Strava" + a fine-print permissions note. A faint Volt radial glow bleeds
from the top.

**Chat (home).** Header: avatar + name + "● Your running coach" status, with debug
and settings icons. Optional pending banner. Scrolling thread with a "Today"
divider chip, coach/runner bubbles, inline cards, and Seen. Pinned composer.

**Plan changes.** Back-titled screen. Intro line, then one card per proposal:
Volt when-label, distance badge, workout name, detail, the coach's **one-line
rationale** in a quiet chip (runner can ask for more in chat), and Reject /
Approve. Empty state: centered check with "Nothing waiting on you."

**Settings.** Sections: **Profile** (display name + Save) and **Connections**
(Strava, Garmin rows with brand icon, status dot, connect/sync/disconnect).
Garmin shows its email/password caveat inline. Destructive actions in `error`.
Sign out at the bottom.

**Full plan (to design).** The one non-chat screen: the week/block ahead. Per-day
rows (rest / easy / workout / long), distance and a one-line focus, today marked,
completed vs upcoming distinguished, race day as the anchor. Phone-first, glanceable,
read-only — edits happen by talking to Naomi. *Not yet built in code.*

## 7. Interaction & motion

- **Acknowledgement, not fake typing.** A runner message shows **delivered**, then
  **Seen** once the coach is working. The **typing-dots** indicator appears **only
  when message text is actually streaming back** — tool calls and background work
  never trigger it. *(Front end is built; SSE backend wiring is a follow-up.)*
- **Auto-scroll** to the newest message; **auto-grow** composer to ~128px.
- **Avatar glow** is the ambient "alive" cue; keep motion minimal and purposeful.

## 8. Out of scope / deferred

History search (Q&A covers recall for now), quick-reply chips (no), in-app Garmin
auth changes (no — needs API approval we aren't seeking). The debug/tool-trace
view is a **dev tool**, to be gated behind a feature flag (e.g. LaunchDarkly /
Statsig / Flagsmith / Unleash), not shipped to runners. Per-change plan approval
is a **short-term trust-building** step on the way to the coach adapting the plan
autonomously (per the brief).

## 9. Implementation notes

- Theme & tokens: `frontend/input.css` → compiled to `go/web/assets/app.css`
  (`make gen-css`). DaisyUI v5 theme name is `naomi`.
- Assets (CSS, fonts, htmx, lucide) are self-hosted and embedded via `go:embed`
  (`go/web/assets.go`), served at `/assets`. No CDN, no FOUC, works offline.
- Screens: `go/web/*.templ`; regenerate with `make gen-templ`.
- Inline cards: `web.Message.Workout` / `web.Message.Stats` (additive, optional).
- Preview any screen: `make screenshots` (renders to `docs/screenshots/`).

## 10. Figma build prompt

Paste into a desktop Claude Code session on this repo (MCP write approvals work
there; an allow-rule is in `.claude/settings.local.json`):

> Create a new Figma design file in my team and mock up the Naomi running-coach
> screens — Login, Chat, Plan changes, Settings, and a new Full-plan (week)
> screen — at 390×844. Use this repo's design system: read `docs/design.md` for
> the spec, `frontend/input.css` for the exact Volt tokens, and match
> `docs/screenshots/*.png` and `go/web/*.templ` precisely. Build a small
> component set first (avatar, coach bubble, runner bubble, workout card, stat
> strip, composer, buttons), then compose the screens from instances.

# AI Running Coach — System Design Document

*Architecture, decision log, and staged build plan*

This document captures the design decisions for an agentic running coach built in Go on Google's Agent Development Kit (ADK). It is the output of a deliberate design-first process: every decision below was reasoned through before any production code. It is intended to travel with the project so the rationale behind each choice is preserved.

---

## 1. Overview and scope

The product is a single conversational running coach that a runner interacts with over months. It analyzes workouts, answers training questions, and eventually generates and adapts training plans. The runner talks to one bot through one chat thread; all structural complexity lives behind that interface and is invisible to them.

**Scale:** personal / small-scale. This is a deliberate constraint that right-sizes several decisions below — throughput and rate-limit machinery is deferred, while safety-critical commitments (correct metrics, evaluation gates) are kept regardless of scale.

### Guiding principles

- **Deterministic core.** All quantitative and rule-based work (training-load math, plan constraints, data retrieval) is plain Go exposed to the agent as tools. The model never computes a number or recalls a fact from memory — it calls a tool. Test: if identical input must yield an identical number, it is a tool.
- **Earned complexity.** Start with one agent and the simplest structure that works. Add machinery (multi-agent routing, summarization, rate-limit queues) only when it demonstrably earns its place.
- **Software system first.** Tracing, evaluation, and version control apply to every layer from the first commit, not as an afterthought.
- **Contained fragility.** External dependencies with weak stability contracts are isolated behind interfaces and treated as best-effort, never load-bearing.

---

## 2. Architecture

The system is three layers: external data sources, a deterministic Go core, and the agent. Data flows in through an ingest path that fetches, normalizes, and stores before the agent ever runs; the agent reads only from the normalized store, never from a raw API payload.

### 2.1 The deterministic Go core

This is the heart of the system and where correctness is enforced. It owns three things: data ingestion and normalization; the metric engine (training stress, acute:chronic workload ratio, pace and heart-rate zones, and related computations); and plan templates with their constraints. Each capability the agent has is backed by tools that call into this core. Because the math lives here, it is unit-testable and reproducible, and the model cannot invent or drift on a number.

### 2.2 The agent

A single ADK llmagent with the full tool surface drives stages 1 and 2. It receives a trigger (a chat message or a newly-synced workout), assembles a lean context, and runs a bounded reasoning loop: the model decides which tool to call, the Go core executes it, the result returns to the model, and the loop repeats until the model produces a final coached response. The loop is hard-capped to prevent runaway tool chains — the most common production failure mode for tool-calling agents.

### 2.3 One chat interface over an evolving backend

The runner always talks to a single thread. Through stages 1 and 2 there is literally one agent behind it. At stage 3, the backend splits into a coordinator that silently routes to specialist sub-agents (analysis, Q&A, planning), but the chat surface does not change at all — the split happens entirely behind the interface boundary. Voice consistency across specialists is guaranteed by a shared base instruction (coach persona, tone, safety rules) that every agent inherits, with each specialist layering only task-specific guidance on top. This instruction hierarchy is designed now, with one agent, so the seams are invisible when the split arrives.

---

## 3. Memory and long-running conversations

The defining challenge for a coach is that the relationship spans months while the context window is finite. The resolving principle: the conversation transcript is not the memory. Durable knowledge of the runner lives in the database and is retrieved on demand; the transcript is ephemeral and pruned. What makes the coach feel like it knows the runner is queryable structured facts, not a long context window.

### 3.1 The four tiers

- **Working context.** Rebuilt every turn, kept deliberately small: system prompt and persona, the rolling summary, the last few exchanges verbatim, and any tool results just fetched. This is the only thing sent to the model.
- **Rolling summary.** Older turns compressed into a short paragraph capturing goals, themes, decisions, and open threads. Updated periodically. Provides conversational continuity at a few hundred tokens.
- **Durable runner store.** The real long-term memory: training history, current plan, load metrics, injuries, preferences, and PRs, held as structured rows in the database and retrieved via tools when relevant.
- **Raw transcript archive.** Every message, in cold storage, never sent to the model wholesale. Source for regenerating summaries and for audit and debugging. Grows forever cheaply.

### 3.2 Fact promotion

The mechanism that turns a casual mention into durable memory is a tool the agent calls live. When the runner says something salient — a goal, an injury, a hard constraint — the agent calls a record-fact tool that writes structured data to the durable store immediately, so the fact survives summarization by construction. Known limitation: with no batch-extraction backstop, the agent will occasionally miss a fact it should have recorded; that fact then lives only in the raw archive. This is an accepted trade for simplicity at this stage and can be backstopped later with a periodic extraction pass over the archive, with no other change.

---

## 4. Decision log

Every architectural decision made during design, with its resolution and the reasoning behind it.

| Decision | Resolution and rationale |
|---|---|
| **Framework** | Google ADK for Go. Chosen because tracing, cost tracking, lifecycle callbacks, and a path to multi-agent come as first-class machinery rather than hand-rolled around a manual loop. |
| **Interface** | One chat thread for the entire months-long relationship. Backend topology is invisible behind it and can change without the interface changing. |
| **Topology** | Single agent for stages 1–2; coordinator-plus-specialists only at stage 3. Splitting is deferred because mixed-intent routing is overhead that is only worth it once specialist instructions have genuinely diverged — which planning forces. |
| **Capability order** | Analysis, then Q&A, then planning. Analysis is most self-contained and rests on the metric engine; planning is last because it carries real stakes. |
| **Memory model** | Four tiers (working context, rolling summary, durable store, cold archive). The transcript is never the memory; structured facts in the DB are. Resolves the false either/or between summary and retrieval — both are needed, doing different jobs. |
| **Fact promotion** | Live record-fact tool the agent calls when it hears something salient. Captured the moment it is mentioned; survives compaction by construction. |
| **Tool vs reasoning** | Quantitative and rule-bound work is Go tools; interpretive and communicative work is the model. The model never computes a metric or recalls a fact. |
| **Tracing & cost** | On ADK callbacks from the first commit. Nearly free to add and the primary means of debugging the agent. |
| **Planning autonomy** | Build a plan-change approval gate (a before-tool callback), default it on, and flip toward full autonomy only on evidence — passing evals and observed agreement between proposed and approved changes. "Trusted" means instrumented, not assumed. |
| **Eval bar** | Block any release on a wrong metric OR unsafe advice. Metrics are checked deterministically against frozen scenarios; advice is checked against a written safety rubric. Both gates are hard. |
| **Strava ingest** | Pull-based. The webhook delivers only an activity ID and acts purely as a trigger: receive event, fetch full detail and streams, normalize, store, then invoke the agent. The agent reads the store, never Strava directly. |
| **Garmin ingest** | Unofficial Garmin Connect client for wellness data (sleep, HRV, stress, body battery, resting HR, readiness). Wrapped behind an interface, treated as best-effort enrichment, never load-bearing. Swappable for the official Health API or an aggregator later at no downstream cost. |
| **Scale** | Personal / small-scale. Defers the Strava rate-limit queue and lets memory start with just the durable store plus a naive recent-window; keeps the tool/reasoning line, tracing, and eval bar regardless. |

---

## 5. Staged build plan

Three stages, with cross-cutting concerns spanning all of them.

1. **Stage 1 — Workout analysis.** A single agent over the single chat interface. Build the Strava ingest path and the metric engine first; the agent interprets computed metrics and coaches in plain language. This is the foundation everything else rests on.
2. **Stage 2 — Conversational Q&A.** The same single agent, extended with retrieval tools so it can reason over the runner's stored history. Still no routing — one agent with more tools handles mixed questions naturally.
3. **Stage 3 — Planning.** Introduce the coordinator and a planning specialist with the approval gate. This is the trigger for the multi-agent split, because plan changes need a distinct instruction set and a human-approval path the other capabilities do not.

### Spanning every stage

- Memory: durable runner store from day one; rolling-summary compaction added when conversations actually get long enough to need it.
- Tracing and cost tracking on every tool and model call.
- An evaluation suite of frozen runner scenarios — the spiking runner, the detraining runner, the consistent runner — with known-correct metric outputs and a written safety rubric.

### Personal-scale simplifications

Because the initial scope is personal, the following are explicitly deferred (not removed): the webhook-to-queue-to-rate-limited-worker machinery for Strava (a simpler synchronous fetch suffices until there is contention against the shared rate limit), and rolling-summary compaction (the durable store plus a recent-message window is adequate until conversations grow long). The design for both is recorded so they can be added when needed.

---

## 6. Risks and things to watch

### Strava API migration (time-sensitive)

Strava is mid-way through a 2026 developer-program change: some endpoints are being retired with a grace period, the API base URL changes in early 2027, and authentication now returns a granted-scope field. Design against the current changelog rather than hardcoding the old host, and treat checking the official changelog as a recurring task. Private activity data requires the read-all scope.

### Garmin client fragility

The unofficial Garmin path rests on an interface with no stability contract and can break on any Garmin-side change. The mitigation is structural: Garmin is isolated behind an interface and treated as best-effort enrichment, so when it breaks the coach keeps working on Strava data and only the Garmin module needs patching. The official Health API or an aggregator can be swapped in behind the same interface at no downstream cost.

### Autonomy progression

Fully autonomous plan changes are the end state, not the starting state. The risk is flipping to autonomy before the evidence justifies it. The discipline: the approval gate stays on, and autonomy is enabled only when the eval suite passes consistently and traces show the coach's proposals match what would have been approved. The gate also remains the fallback if a model update regresses.

---

*End of design document. This plan was developed design-first; implementation follows against the staged build plan in Section 5.*

---

## Amendment A — Measurement vs. judgement (supersedes the determinism axiom where they conflict)

### What we got wrong

§1 states the determinism test as: *"if identical input must yield an identical number, it is a tool."* That test is too coarse. It conflates two unlike things that both happen to be reproducible:

- **Measurements** — quantities with a ground truth, where the model would be simply *wrong* to vary: total km this week, splits, pace, HR drift, weekly aggregates, time-in-zone. Here determinism buys **correctness**. This is the failure the coach actually had in practice (a model mis-summing a week's mileage), and the guardrail is right.
- **Judgements** — quantities with no ground truth, where a formula is one defensible guess among many: a race-time projection, a readiness verdict, ACWR read as a go/no-go gate. Here determinism buys **false confidence**. Making such a thing a tool launders an opinion into a fact and strips the coach of its ability to read *this* athlete in *this* context.

A Riegel prediction passes the original test — same input, same number — so the axiom *forces* it into the deterministic core. But Riegel is a population regression off a single best effort with a fixed exponent; it ignores training state, fitness trend, the specific goal, how recent long runs actually held up. It emits a "correct" number that is frequently wrong about the individual, and worse, it presents as authoritative and crowds out the coach's contextual read. That rigidity — not Riegel specifically, but the whole class of judgement-dressed-as-measurement — is what made the metric-rich coach read the athlete *worse* than the earlier metric-free one.

### The corrected line

Draw the deterministic boundary at **measurement vs. judgement**, not "number vs. prose." The test becomes:

> **Is there a ground-truth answer this value could be *wrong* about?** If yes, it is a measurement → compute it deterministically and hand it over. If no, it is a judgement → it belongs to the coach, grounded in the measurements it is handed.

Consequences:

- **Measurements stay deterministic tools.** Aggregation, pace, splits, drift, decoupling, zone distributions, CTL/ATL/TSB, ramp — all computed, all quoted, never re-derived by the model. The anti-arithmetic guardrail is a *measurement-integrity* rule and is kept in full; it is not the source of rigidity.
- **Judgements move to the coach.** Projection, readiness, and "push or back off" are the coach's call, reasoned over the data snapshot and stated with what each call is based on. The projection of record is prose committed via `set_projection`, never a formula's output — which is already how the build works.
- **Formulas like Riegel/ACWR are demoted, not deleted.** They may be surfaced as *one optional reference input* the coach can cite and contextualize ("textbook math says ~X off your 5k, but given Saturday faded I'd hold you at Y"). They are never the authoritative number and never a hard gate.
- **"Science-based" is reframed.** Rigor lives in *grounding and traceability* — every judgement accountable to real measured data and saying what it rests on — not in replacing the coach's judgement with a regression.

### Why trust this over the original deduction

§1's axiom was reasoned out design-first, before code. The metric-free coach is a *working artifact* that demonstrably read the athlete better. When a clean deduction and a working artifact disagree, amend the deduction. The metric engine is not thrown out; it is put back in its lane — a **measurement engine that feeds context**, not a judgement engine that emits verdicts.
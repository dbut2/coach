# 0001 — Measurement vs judgement is the deterministic boundary

- **Status:** Accepted
- **Date:** 2026-06-25
- **Supersedes:** the determinism axiom in [`brief.md`](../brief.md) §1, where
  they conflict

## Context

brief.md §1 set the determinism test as: *"if identical input must yield an
identical number, it is a tool."* That test conflates two unlike things that are
both reproducible:

- **Measurements** — quantities with a ground truth the model would be simply
  *wrong* to vary: weekly distance, splits, pace, HR drift, time-in-zone. Here
  determinism buys **correctness**. The coach's real-world failure — a model
  mis-summing a week's mileage — is exactly this, and the guardrail is right.
- **Judgements** — quantities with no ground truth, where a formula is one
  defensible guess among many: a race-time projection, a readiness verdict, ACWR
  read as a go/no-go gate. Here determinism buys **false confidence**: it
  launders an opinion into a fact and strips the coach of its ability to read
  *this* athlete in *this* context.

A Riegel projection passes the original test — same input, same number — so the
axiom *forces* it into the deterministic core. But Riegel is a population
regression off a single best effort with a fixed exponent; it ignores training
state, fitness trend, the specific goal, and how recent long runs actually held
up. It emits an authoritative-looking number that is frequently wrong about the
individual and crowds out the coach's contextual read. That rigidity — the whole
class of judgement-dressed-as-measurement — is what made the metric-rich coach
read the athlete *worse* than the earlier metric-free one.

## Decision

Draw the deterministic boundary at **measurement vs. judgement**, not "number
vs. prose." The test becomes:

> Is there a ground-truth answer this value could be *wrong* about? If yes, it
> is a measurement — compute it deterministically and hand it over. If no, it is
> a judgement — it belongs to the coach, grounded in the measurements it is
> handed.

## Consequences

- **Measurements stay deterministic tools.** Aggregation, pace, splits, drift,
  decoupling, zone distributions, CTL/ATL/TSB, ramp — all computed, all quoted,
  never re-derived by the model. The anti-arithmetic guardrail is a
  *measurement-integrity* rule and is kept in full; it was never the source of
  the rigidity.
- **Judgements move to the coach.** Projection, readiness, and "push or back
  off" are the coach's call, reasoned over the data snapshot and stated with
  what each rests on. The projection of record is prose committed via
  `set_projection`, never a formula's output.
- **Formulas like Riegel/ACWR are demoted, not deleted.** They may be surfaced
  as *one optional reference input* the coach can cite and contextualise
  ("textbook math says ~X off your 5k, but given Saturday faded I'd hold you at
  Y"). Never the authoritative number, never a hard gate.
- **"Science-based" is reframed.** Rigour lives in *grounding and traceability*
  — every judgement accountable to measured data and saying what it rests on —
  not in replacing the coach's judgement with a regression.

## Alternatives considered

Keeping the original "number vs. prose" axiom. Rejected on evidence: it was
reasoned out design-first, before code, while the metric-free coach is a working
artifact that demonstrably read the athlete better. When a clean deduction and a
working artifact disagree, amend the deduction. The metric engine is not thrown
out — it is put back in its lane: a **measurement engine that feeds context**,
not a judgement engine that emits verdicts.

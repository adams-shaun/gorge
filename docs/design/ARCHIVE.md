# Archived design records

On 2026-10-16 the historical design records that used to live in
`docs/superpowers/` (specs, plans, reports, evidence and the agent-seat
roster) left the tree, per
[ADR 0001](../decisions/0001-repo-layout.md) decision 9. The documents that
code or agents still consult moved to this directory, `docs/design/`, under
their original file names. Everything listed below was parked.

**Where the full text lives.**

- In git history: every file below is present at commit `6d95c6a54`, the
  parent of the commit that removed them. Read one with
  `git show 6d95c6a54:docs/superpowers/<path>`, where `<path>` is the File
  column below.
- On the operator's box: a copy of every file, with the same relative
  layout, is in the gitignored `.superpowers/archive/` directory of the main
  checkout (agentctl links `.superpowers` into each worktree).

A reference in code or docs to a parked file now reads
`archived <file name> (docs/design/ARCHIVE.md)`.

The 15 lab-only design docs that lived in `docs/design/` (the XMage
compliance oracle and Level B, the XMage scenario port, the draft-events
design and its E1/E3/E5/E5-d plans, draft RL, the az-c3/az-c4/az-c5 hosting
notes and the PlayLines pl-3/pl-4/pl-8 notes) moved to four-yay's
`docs/design/` (private) on 2026-10-16, under the same file names; see
[ADR 0001](../decisions/0001-repo-layout.md)'s amendment.

Outcomes are one-line summaries taken from each document's own status,
summary or verdict lines; read the document before relying on one.

The `evidence/token-art/` directory also held 8 screenshots
(`before-*.png`, `after-*.png`) referenced by its `REPORT.md`.

## Specs

| File | Date | Title | Outcome |
|---|---|---|---|
| `specs/2026-09-03-mtgcore-go-engine-design.md` | 2026-09-03 | mtgcore — a native Go Magic rules engine | Founding engine design (M0-M1); the milestone table was superseded by the post-M1 roadmap. |
| `specs/2026-09-04-gorge-m2a-tables-spectators-design.md` | 2026-09-04 | gorge M2a — tables and spectators | Tables and spectators design; implemented. |
| `specs/2026-09-04-gorge-post-m1-roadmap.md` | 2026-09-04 | gorge — post-M1 roadmap | Post-M1 milestone roadmap; superseded by the later programmes. |
| `specs/2026-09-05-gorge-m2bc-library-first-plan.md` | 2026-09-05 | gorge M2b + M2c — the library-first follow-up plan | M2b/M2c library-first plan; executed. |
| `specs/2026-09-05-gorge-m2d-fixture-design.md` | 2026-09-05 | gorge M2d (decision-kind completeness) + M2e (local fixture) — design | M2d/M2e design: gorge as a library, `cmd/gorged` as the local fixture. |
| `specs/2026-09-06-client-capability-registry.md` | 2026-09-06 | Client capability registry — what the UI will need, and what supports it today | UI need registry; no longer maintained. |
| `specs/2026-09-06-client-design-system.md` | 2026-09-06 | gorge client — design plan | First client design plan; its layout was superseded by the 2026-09-28 table UI rework. |
| `specs/2026-09-06-gorge-learned-policy-research.md` | 2026-09-06 | What a real learned policy for gorge would have to be — research, R1 | Research R1: what a learned policy would need; measured ~17.5 ms per bot game. |
| `specs/2026-09-17-compiled-face-metadata-design.md` | 2026-09-17 | Compiled Face Metadata Design | Integer-backed compiled face catalog; implemented. |
| `specs/2026-09-17-rules-test-parallelism-design.md` | 2026-09-17 | Rules Test Parallelism Design | Rules test parallelism design; superseded by the per-test budget and sharding. |
| `specs/2026-09-17-search-land-isolation-proposal-design.md` | 2026-09-17 | Search Probe Land-Isolation Proposal Design | Land-isolation proposal for the search sampler; implemented. |
| `specs/2026-09-17-search-land-mixture-weight-cap-design.md` | 2026-09-17 | Search Probe Land-Mixture Weight-Cap Design | Rejected: the 50/50 mixture was kept. |
| `specs/2026-09-17-search-probe-profile-optimization-design.md` | 2026-09-17 | Search-probe profile optimization design | Search-probe profile optimisation design; executed. |
| `specs/2026-09-17-search-proposal-coverage-design.md` | 2026-09-17 | History-Conditioned Proposal Coverage Design | History-conditioned proposal coverage; implemented. |
| `specs/2026-09-17-search-world-sampler-design.md` | 2026-09-17 | History-conditioned search probe design | History-conditioned search probe design; implemented. |
| `specs/2026-09-18-agentctl-library-design.md` | 2026-09-18 | agentctl — moved | Stub: agentctl design moved to its own repo. |
| `specs/2026-09-18-botbench-decision-trace-design.md` | 2026-09-18 | Botbench decision-trace design | Botbench JSONL decision trace; implemented. |
| `specs/2026-09-18-compiled-predicate-cost-design.md` | 2026-09-18 | Compiled Predicate and Cost Design | Compiled filter/cost predicates; implemented. |
| `specs/2026-09-19-hosted-bot-policy-selection-design.md` | 2026-09-19 | Hosted bot policy selection | Hosted bot selection by name; implemented, then extended by hosted-bot-packages. |
| `specs/2026-09-23-seat-box-pill-requirements.md` | 2026-09-23 | Seat-box "pill" visual requirements — fb-20260923T015756Z-c1c7d6d8 | Visual requirements for seat-box pills from a player report; implemented in `web/`. |
| `specs/2026-09-28-client-table-ui-rework-design.md` | 2026-09-28 | Client table UI rework — design | Table UI rework; sub-project plans executed in `web/`. |
| `specs/2026-09-28-loop-shortcut-protocol-design.md` | 2026-09-28 | Loop shortcut protocol, execution and replay: the deferred half of the loops design | Deferred half of the loops design (consent, execution, replay); not started. |
| `specs/2026-09-28-loops-and-shortcuts-design.md` | 2026-09-28 | Loops and shortcuts: a bounded, replayable execution protocol | Loops and shortcuts proposal; client-driven v1 only. |
| `specs/2026-09-28-manabrew-protocol-scope.md` | 2026-09-28 | ManaBrew protocol support: scoping spec | ManaBrew protocol scoping; the wire landed, then deprioritised 2026-10-02. |
| `specs/2026-09-28-scam-exe-combo-lines.md` | 2026-09-28 | Scam.EXE: combo audit and real-engine loop experiments | Scam.EXE combo audit and loop experiments. |
| `specs/2026-09-28-window-diagnostics-scoping.md` | 2026-09-28 | Window diagnostics scoping — operator decision and B1 implementation brief | Window diagnostics scoping; client half landed, server half briefed. |
| `specs/2026-09-29-cross-engine-oracle-audit.md` | 2026-09-29 | Cross-engine oracle audit — plan (2026-09-29) | Cross-engine (Forge) oracle audit plan; superseded by the XMage oracle design. |
| `specs/2026-09-30-gorge-searchbench-design.md` | 2026-09-30 | Gorge-native FDN search benchmark — design (2026-09-30) | Native FDN search benchmark design; executed (2026-10-01 report). |
| `specs/2026-10-06-legal-walk-design.md` | 2026-10-06 | Priority legal-action walk: design and implementation plan | Priority legal-action walk design; steps S0-S4 landed. |
| `specs/2026-10-06-pointer-free-corpus-design.md` | 2026-10-06 | Pointer-free card corpus: design and implementation plan | Pointer-free card corpus to shrink GC scan set; `internal/corpusheap` guards it. |
| `specs/2026-10-07-mana-member-carry-design.md` | 2026-10-07 | Mana-member carry (legal-walk §S4) — design | Legal-walk S4 mana-member carry; landed (eb31d3f1a). |
| `specs/2026-10-07-mzenc-design.md` | 2026-10-07 | mzenc: a byte-identical Go port of MageZero v0.2's `StateEncoder` | Byte-identical Go port of MageZero's StateEncoder; now four-yay's `training/mzenc` (private). |
| `specs/2026-10-07-searchprobe-engine-reset-design.md` | 2026-10-07 | Search-probe engine reset: pooled replay instead of clone | Pooled replay instead of clone; landed. |
| `specs/2026-10-07-smoke-perf-target-design.md` | 2026-10-07 | Smoke perf target design | Opt-in base-vs-candidate smoke perf target (`scripts/smoke-perf.sh`, since moved to four-yay). |

## Plans

| File | Date | Title | Outcome |
|---|---|---|---|
| `plans/2026-09-03-mtgcore-m0-m1.md` | 2026-09-03 | mtgcore M0 + M1 Implementation Plan | Executed: M0 and M1 landed on main (2fe9c72). |
| `plans/2026-09-04-gorge-m2a-tables-spectators.md` | 2026-09-04 | gorge M2a — Tables & Spectators Implementation Plan | Executed: tables and spectators exist in `server/host`. |
| `plans/2026-09-05-gorge-m2r-ratchet-to-zero.md` | 2026-09-05 | gorge M2r — Ratchet to Zero Implementation Plan | Executed: the repo-deck ratchet was driven down (now 2 cards). |
| `plans/2026-09-06-gorge-m2d-decision-kinds.md` | 2026-09-06 | gorge M2d — Decision-Kind Completeness Implementation Plan | Executed: `mulligan` and `modes` decision kinds landed. |
| `plans/2026-09-17-compiled-face-metadata.md` | 2026-09-17 | Compiled Face Metadata Implementation Plan | Executed through Task 8 (see the 2026-09-17 resume report). |
| `plans/2026-09-17-rules-test-parallelism.md` | 2026-09-17 | Rules Test Parallelism Implementation Plan | Plan for the rules test parallelism design; outcome not recorded in the plan. |
| `plans/2026-09-17-search-land-isolation-proposal.md` | 2026-09-17 | Search Probe Land-Isolation Proposal Implementation Plan | Executed: the 50/50 land-isolation mixture is the committed sampler. |
| `plans/2026-09-17-search-probe-profile-optimization.md` | 2026-09-17 | Search-probe Profile Optimization Implementation Plan | Ten measured search-probe optimisation iterations; superseded by later search work. |
| `plans/2026-09-17-search-proposal-coverage.md` | 2026-09-17 | History-Conditioned Proposal Coverage Implementation Plan | Executed: history-conditioned proposal coverage in the search probe. |
| `plans/2026-09-17-search-world-sampler.md` | 2026-09-17 | History-conditioned Search Probe Implementation Plan | Executed: the runnable history-conditioned search probe. |
| `plans/2026-09-18-botbench-decision-trace.md` | 2026-09-18 | Botbench Decision Trace Implementation Plan | Executed: the botbench decision trace (see the 2026-09-19 AR7 report). |
| `plans/2026-09-18-compiled-predicate-cost.md` | 2026-09-18 | Compiled Predicate and Cost Implementation Plan | Executed; see the 2026-09-18 running report. |
| `plans/2026-09-18-performance-and-bot-effectiveness-strategy.md` | 2026-09-18 | Performance and bot-effectiveness strategy | Strategy note for throughput and bot quality; superseded by the hosted-bot and AZ programmes. |
| `plans/2026-09-19-hosted-bot-policy-selection.md` | 2026-09-19 | Hosted Bot Policy Selection Implementation Plan | Executed; verified by the 2026-09-19 report. |
| `plans/2026-09-19-learned-cast-profile.md` | 2026-09-19 | Learned cast profile — plan (L-series) | L-series ran; the fitted L4 profile failed its +3pp gate (50.92%). |
| `plans/2026-09-27-alphazero-mcts.md` | 2026-09-27 | AlphaZero-style MCTS (tickets 1–2) Implementation Plan | Executed tickets 1-2: `bots/azmcts` PUCT core and engine adapter. |
| `plans/2026-09-28-flow-profiles-keymap.md` | 2026-09-28 | Flow Profiles, Breakpoints and Keymap Implementation Plan | Client lane plan (flow profiles, breakpoints, keymap); landed in `web/`. |
| `plans/2026-09-28-ui-layout.md` | 2026-09-28 | Layout Profiles and Board Visuals Implementation Plan | Client lane plan (layout profiles, board visuals); landed in `web/`. |
| `plans/2026-09-28-ui-manabrew-adapter.md` | 2026-09-28 | ManaBrew Client Adapter Implementation Plan | Client ManaBrew adapter plan; ManaBrew work deprioritised 2026-10-02. |
| `plans/2026-09-28-ui-motion.md` | 2026-09-28 | Client Model Transitions and Motion Implementation Plan | Client lane plan (model transitions, motion); landed in `web/`. |
| `plans/2026-09-28-ui-prompts.md` | 2026-09-28 | Prompt System (Lane C) Implementation Plan | Client lane C plan (prompt system); landed in `web/`. |
| `plans/2026-09-30-searchbench-replication.md` | 2026-09-30 | Gorge-native replication of DraftZero experiment #3 (search benchmark) | Executed; results in the 2026-10-01 searchbench report. |
| `plans/2026-10-02-magezero-style-training-checklist.md` | 2026-10-02 | Checklist: MageZero-style training on gorge | Checklist only; nothing scheduled (the work moved to the side branch `mzrepro`). |
| `plans/2026-10-02-xmage-oracle-foundation.md` | 2026-10-02 | XMage Compliance Oracle — Foundation (X0–X3) Implementation Plan | Executed X0-X3; the oracle now lives in four-yay's `oracle/` (private). |
| `plans/2026-10-07-mana-member-carry.md` | 2026-10-07 | Mana-member carry (legal-walk §S4) Implementation Plan | Executed: legal-walk step S4 landed (eb31d3f1a). |
| `plans/2026-10-07-mzenc-hash-port.md` | 2026-10-07 | mzenc hash-port Implementation Plan (Milestone 1) | Executed milestone 1: the mzenc hash port (now four-yay's `training/mzenc`). |
| `plans/2026-10-07-mzenc-state-walkers.md` | 2026-10-07 | mzenc StateEncoder walkers Implementation Plan (Milestones 2–3) | mzenc StateEncoder walkers (milestones 2-3); mzenc is used only for ablations. |
| `plans/2026-10-07-searchprobe-engine-reset.md` | 2026-10-07 | Search-Probe Engine Reset Implementation Plan | Executed; see the 2026-10-07 measured-outcome report. |
| `plans/autopay-v1-hardening/README.md` | 2026-09-26 | Auto-pay V1 hardening wave (2026-09-26) | Auto-pay V1 hardening wave of 17 ticket briefs (below); wave filed through agentctl. |
| `plans/autopay-v1-hardening/aph-cast-shape-gate.md` | 2026-09-24 | Auto-pay: withhold plans from casts whose cost the witness cannot bind | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-combo-chosen-identity.md` | 2026-09-24 | Auto-pay: admit "Add {X} or {Y}" (Combo), recorded-colour (Chosen) and commander colour-identity producers | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-hand-reserve.md` | 2026-09-24 | Auto-pay: hand-aware tie-break (keep the colours the rest of your own hand needs) | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-interference-scope.md` | 2026-09-24 | Auto-pay: scope mana/tap interference to the sources it can affect; one global reason for the rest | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-last-resort-plans.md` | 2026-09-24 | Auto-pay: last-resort plans (sacrifice-self, life, damage, doesn't-untap, return-to-hand) with disclosed consequences | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-last-resort-web.md` | 2026-09-24 | Web auto-pay: disclose last-resort consequences; confirm life payments from every entry point | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-lazy-offers.md` | 2026-09-24 | Auto-pay: build PaymentActions lazily, only for consumers (53-60% of engine CPU today) | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-mirror-in-fuzz-bench.md` | 2026-09-24 | Auto-pay: run the A/B payment mirror inside cardfuzz and botbench (opt-in) | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-offer-one-pass.md` | 2026-09-24 | Auto-pay: build every offer from one candidate walk and the pending Options | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-plan-diagnostics.md` | 2026-09-25 | Auto-pay: surface planner outcomes, details, nodes and fallbacks (engine sink + botbench report) | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-potential-granted-intrinsics.md` | 2026-09-24 | Auto-pay: PotentialMana must see type-granted intrinsic mana (Urborg, Prismatic Omen) | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-pp-coverage-port.md` | 2026-09-25 | Auto-pay: port the passing PP-matrix audit tests and make PP-20 assert planned casts succeed | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-pp-matrix-acceptance.md` | 2026-09-24 | Auto-pay hardening: integrated acceptance, PP-01..PP-28 matrix and sweep report | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-producer-tiers.md` | 2026-09-24 | Auto-pay: classify every mana ability into normal / last-resort / deferred; plan from normal only | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-rank-order.md` | 2026-09-24 | Auto-pay: rank plans by cost tier, then creatures, then sources (Arena/Forge order); numeric tie-break | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-rank-search.md` | 2026-09-24 | Auto-pay: rank-aware bounded search (unit classes, pips first, branch-and-bound); O(1) incarnation lookup | Ticket brief in the auto-pay V1 hardening wave. |
| `plans/autopay-v1-hardening/aph-web-autopass.md` | 2026-09-24 | Web auto-pay: plan-only casts are not an empty window; own-spell auto-pass follows the seat preference | Ticket brief in the auto-pay V1 hardening wave. |

## Reports

| File | Date | Title | Outcome |
|---|---|---|---|
| `reports/2026-09-03-m0-m1/deferred-items.md` | 2026-09-03 | Deferred minors, parked findings and final-wave items (digest from the ledger) | M0/M1 deferred minors and final-wave items, digested from the SDD ledger. |
| `reports/2026-09-03-m0-m1/final-rereview.md` | 2026-09-03 | Final fix-wave re-review — gorge, base 5db64f2 → head 2fe9c72 | Final fix-wave re-review: findings addressed before the M1 merge. |
| `reports/2026-09-03-m0-m1/final-review.md` | 2026-09-03 | Final whole-branch review — gorge @ 5db64f2 (75 commits, root eaeca1e) | Whole-branch review of M0/M1: ready to merge with fixes. |
| `reports/2026-09-03-m0-m1/interface-comparison.md` | 2026-09-03 | Client↔engine interface: mtgplay (A, production) vs gorge (B, in development) | mtgplay vs gorge client interface comparison; informed the closed decision-kind design. |
| `reports/2026-09-03-m0-m1/rulings.md` | 2026-09-03 | M0/M1 rulings digest | Digest of every M0/M1 orchestrator ruling. |
| `reports/2026-09-03-m0-m1/ui-fix-history.md` | 2026-09-03 | Was the client↔engine interface a primary cause of the play client's misses? | Measured whether the old interface caused the play client's misses. |
| `reports/2026-09-03-m0-m1/ui-inspiration.md` | 2026-09-03 | UI inspiration for the `gorge` browser client | UI survey that shaped the first browser client. |
| `reports/2026-09-06-ds4-task-m1-botbench-matrix.md` | 2026-09-06 | Task M1 — a deck-pair matrix in `cmd/botbench` | Seat task: deck-pair matrix added to botbench. |
| `reports/2026-09-07-ds4-task-dp0-decision-histogram.md` | 2026-09-07 | Task dp0 — measure which decisions the bot actually faces | Measured which decisions the bot faces; attackers ranked highest leverage. |
| `reports/2026-09-07-ds4-task-dp1b-policy-determinism.md` | 2026-09-07 | dp1b — deterministic decisions and hand retention | Seat task: deterministic decisions and hand retention in botpolicy. |
| `reports/2026-09-07-ds4-task-u3-format-cost.md` | 2026-09-07 | Task U3 — measure why Commander tables run slower than constructed | The ~2x Commander slowdown was not explained by decision count, engine cost or step count. |
| `reports/2026-09-08-ds4-task-fx37-departed-chooser.md` | 2026-09-08 | Task fx37 — pin the departed-chooser resumption event stream | Seat task: pinned the departed-chooser resumption event stream. |
| `reports/2026-09-17-compiled-face-metadata-resume.md` | 2026-09-17 | Compiled face metadata completed checkpoint | Checkpoint: compiled face metadata implemented through Task 8. |
| `reports/2026-09-17-compiled-face-metadata-running.md` | 2026-09-17 | Compiled face metadata running report | Running measurements for compiled face metadata. |
| `reports/2026-09-17-engine-profile-optimization-resume.md` | 2026-09-17 | Engine profile optimization checkpoint and next-task resume | Ten engine-only optimisation iterations completed; nine kept. |
| `reports/2026-09-17-hotspot-optimization.md` | 2026-09-17 | Hotspot optimization: first measured batch | First hotspot batch landed with no policy, event or golden change. |
| `reports/2026-09-17-hotspot-resume.md` | 2026-09-17 | Fresh-session resume prompt: return to the MCTS training question | Session resume prompt (MCTS training question); no findings of its own. |
| `reports/2026-09-17-search-probe-pilot.md` | 2026-09-17 | Search-label probe: 500-game feasibility pilot | 500-game search-label pilot ran; the planned strength comparison did not. |
| `reports/2026-09-17-search-probe-resume.md` | 2026-09-17 | Resume prompt: continue post-rebase search-probe diagnosis | Session resume prompt (search-probe diagnosis); no findings of its own. |
| `reports/2026-09-17-search-probe-running.md` | 2026-09-17 | Search probe: final history-conditioned coverage calibration | Final history-conditioned coverage calibration over 500 roots. |
| `reports/2026-09-17-trigger-pruning.md` | 2026-09-17 | Conservative trigger eligibility pruning | Conservative trigger-eligibility pruning landed. |
| `reports/2026-09-18-compiled-predicate-cost-running.md` | 2026-09-18 | Compiled predicate and cost running report | Running measurements for compiled predicates and costs. |
| `reports/2026-09-19-bot-policy-decision-trace-and-ar7.md` | 2026-09-19 | Bot policy decision trace and AR7 evaluation | AR7 lethal pressure measured +1.65pp held-out and promoted to the default bot. |
| `reports/2026-09-19-hosted-bot-policy-selection.md` | 2026-09-19 | Hosted bot policy selection — verification report | Hosted tables persist `bot_policy`; no strength claim made. |
| `reports/2026-09-19-search-teacher-spike.md` | 2026-09-19 | Search-teacher spike: ensemble determinization over the history-conditioned sampler | Go: ensemble search works as an offline labelling teacher with a confidence margin. |
| `reports/2026-09-24-pn14-explore-entity-outcome.md` | 2026-09-24 | pn14: exploration, per-card entity encoding, outcome-trained PPO (2026-09-24) | Exploration, entity encoding and outcome PPO: no gain over the bot. |
| `reports/2026-09-24-pn20-hindsight-step1.md` | 2026-09-24 | pn20 step 1: hindsight branch mining — measurement (2026-09-24) | KILL (confounded): 94.8% of branched decisions hit `no_world`. |
| `reports/2026-09-24-pn21-known-card-projection.md` | 2026-09-24 | pn21: known-card projection for search sampling (2026-09-24) | Known-card projection for search sampling (`searchprobe.ProjectKnownCards`). |
| `reports/2026-09-24-pn22-mana-tap-search.md` | 2026-09-24 | PN22: mana tap choices in sampled-world search | Opt-in search over bare mana taps; measured, not adopted. |
| `reports/2026-09-24-training-approaches-summary.md` | 2026-09-24 | Training approaches tried for gorge's bot — summary (2026-09-24) | Summary of every bot training attempt to date: only decision-time search (PIMC) gave an edge. |
| `reports/2026-09-25-cast-payment-plans-acceptance.md` | 2026-09-25 | Cast payment plans acceptance audit | PP-matrix acceptance audit of cast payment plans; passed with noted gaps. |
| `reports/2026-09-28-spellbench-fdn-limited.md` | 2026-09-28 | SpellBench FDN Limited in gorge — definition, coverage, backlog | Defined FDN Limited for gorge; coverage and backlog. |
| `reports/2026-09-28-spellbench-policy-networks.md` | 2026-09-28 | SpellBench policy networks — survey and build plan (v1c, 2026-09-28) | Survey and build plan for SpellBench policy networks (v1c). |
| `reports/2026-09-28-spellbench-theory-and-references.md` | 2026-09-28 | SpellBench agent — theory and references (2026-09-28) | Theory and references companion to the SpellBench agent design. |
| `reports/2026-09-29-adopt-a-deck-and-rl-dashboard.md` | 2026-09-29 | Adopt-a-deck: a standing RL target, and a real win-rate dashboard for it | The dashboard already existed in `traindash`; named a standing adopt-a-deck target. |
| `reports/2026-09-30-gate-cpu-ceiling.md` | 2026-09-30 | Gate CPU ceiling — cli-20260930T021924Z-01e737bb | Gate CPU ceiling: wall time unchanged, CPU burst cut sharply. |
| `reports/2026-09-30-gorge-engine-speed.md` | 2026-09-30 | gorge engine speed on draft-zero's docs/015 metrics (2026-09-30) | Re-measured gorge on draft-zero docs/015 speed rows. |
| `reports/2026-09-30-repo-constructed-gauntlet.md` | 2026-09-30 | Repo-constructed gauntlet: every supported constructed deck through SpellBench, and what the RL dashboard can see | Repo constructed decks added as a SpellBench catalog. |
| `reports/2026-09-30-traindash-rescope.md` | 2026-09-30 | traindash re-scope: measure TRAIN_ROOT, fix the empty-panel defect, decide the viewer question | traindash re-scope: fixed the empty-panel defect, decided the viewer question. |
| `reports/2026-10-01-searchbench-replication.md` | 2026-10-01 | Search benchmark replication on gorge (DraftZero experiment #3) | Replicated DraftZero experiment #3 (search vs 17lands agreement) on gorge. |
| `reports/2026-10-07-dzgorge-benchmark.md` | 2026-10-07 | dzgorge self-play benchmark | dzgorge self-play games/hour benchmark recipe and POC result. |
| `reports/2026-10-07-heap-object-attack-poc.md` | 2026-10-07 | Heap-object attack — POC results (2026-10-07) | Heap-object reduction POC on the dzgorge workload. |
| `reports/2026-10-07-searchprobe-engine-reset.md` | 2026-10-07 | Search-probe engine reset: measured outcome | Pooled replay instead of clone landed; output-identical. |
| `reports/2026-10-09-bot-consolidation-plan.md` | 2026-10-09 | Bot policy consolidation plan | Plan to consolidate heuristic bots into four tiers; not implemented. |
| `reports/2026-10-09-common-sense-rules-evaluation.md` | 2026-10-09 | Common-sense rules for the heuristic bots: evaluation | A shared common-sense layer removes one large class of bad actions; became the opt-in `bots/sanity`. |
| `reports/2026-10-09-dumb-choice-census.md` | 2026-10-09 | Dumb-choice census for the heuristic bots | Dumb-choice census of the heuristic bots; stopped early by the operator. |
| `reports/2026-10-10-corpus-coverage-census.md` | 2026-10-10 | Corpus coverage census (2026-10-10) | Which (card, ability) pairs bot play reaches; `cmd/corpuscov` (now four-yay's `cmd/corpuscov`). |
| `reports/2026-10-10-encoding-ablation.md` | 2026-10-10 | Encoding ablation: which state symbols carry signal (2026-10-10) | Ablation of which state symbols carry signal for the policy net. |
| `reports/2026-10-10-mana-model-gap.md` | 2026-10-10 | Mana-model gap: how often the source census withholds a payable cast | How often the source census withheld a payable cast. |
| `reports/2026-10-12-mana-model-ab.md` | 2026-10-12 | Mana-model A/B: do bots choose the newly offered options, and does the flag change results? | Bots do choose the newly offered mana options (5-23% of exposed windows). |
| `reports/2026-10-13-sb-tactical-commander-gate.md` | 2026-10-13 | sb-tactical in 4-seat Commander: the runnable gate and its pre-registered pass rule | sb-tactical runs 4-seat Commander (20/20 games, no stalls); pass rule pre-registered. |
| `reports/2026-10-15-draft-belief.md` | 2026-10-15 | Draft-informed opponent belief for the az search bots | A drafting seat sees ~80% of each opponent's nonland deck; used for az search beliefs. |
| `reports/az-stage0-template.md` | 2026-09-27 | AZ stage 0: gen-0 clairvoyant MCTS vs bot (2026-09-2X) | Template for the AZ stage-0 report; its script was deleted. |

## Evidence

| File | Date | Title | Outcome |
|---|---|---|---|
| `evidence/token-art/REPORT.md` | 2026-10-15 | Token card art renders blank in the web UI — root cause, fix, and proof | Token art rendered blank in the web UI; root cause and fix with before/after screenshots (8 PNGs archived alongside). |

## Agent guide

| File | Date | Title | Outcome |
|---|---|---|---|
| `agent-seats.md` | 2026-09-19 | Agent seats available for gorge work | Seat roster as of 2026-09-19; superseded by the `[[tiers]]` in `.agentctl/config.toml` (2026-09-24). |

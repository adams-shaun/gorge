# Optional payment plans on cast actions

Status: implementation specification, ready for agentctl intake.
Scope: one suggested payment plan per supported cast; client selection per action.
Source inspection baseline: gorge `b4e52200d56210a08fdddc6b4a7ffc8e2236c477`.

## 1. Outcome and decisions

A player can select a spell together with a concrete plan for paying its mana
cost. The engine supplies the plan and executes the selected mana abilities at
the normal payment stage. The client may use suggestions for one cast and pay
manually for the next, including within the same turn.

For a spell costing `{1}{U}{B}`, the UI can display:

```text
Cast X
  Suggested: Island -> U, Swamp -> B, Mountain -> R
  Future alternative: Island -> U, Swamp -> B, Badlands -> R
```

The Mountain plan ranks ahead of the otherwise equivalent Badlands plan because
it preserves the source that can produce two colors. The first release returns
zero or one plan. The wire uses a list so a later release can return alternatives
without multiplying the top-level cast actions.

The accepted product decisions are:

* Selection belongs to an individual cast intent. There is no game-wide engine
  auto-payment mode and no toggle-change event.
* The client owns an `Auto-pay mana` preference, initially off, changeable while
  a priority decision is pending. It chooses whether a click includes a plan.
* Enabling the preference never casts, passes priority, or taps by itself.
* A selected plan names exact sources and ability/color choices. The engine
  cannot silently replace it with its current favorite plan.
* Ordinary manual decisions and intents keep their existing meaning. A missing
  plan means that automatic payment is unavailable for that action, not that
  the action or card is unsupported by gorge.
* All actual activation, payment and game-state changes use the existing rules
  continuations and events.Apply path.

This spec supersedes the earlier conversational proposal for a fixed per-game
`auto_v1` configuration. It also refines the illustrative `choices:[7]` plus
`payment_plan_id` request: the additive selector below is necessary for casts
which are absent from the legacy option list until mana is floated.

## 2. Existing implementation and the compatibility boundary

Read these paths before editing; symbols, not the inspected line numbers, are
the navigation anchors because other tickets are landing continuously:

| Concern | Current source |
|---|---|
| Client choice vocabulary and validation | `decision/decision.go`: Option, Decision, Intent, Validate |
| Priority actions and hypothetical mana offers | `rules/legal.go`: legalActionsPriced, PotentialActions |
| Shared cast feasibility | `rules/mana.go`: offerCastableUsing; `rules/statics.go`: manaFeasibleDescriptor |
| Source alternatives | `rules/mana_available.go`: windowManaUnit, windowManaAlt, windowManaUnits |
| Existing source search | `rules/unless_payment.go`: unlessManaReachable |
| Cast transaction | `rules/cast.go`: beginCast, continueCast, manaWindowAsk, payCast |
| Real mana activation | `rules/mana_activation.go`: availableManaAbilitiesForWindow, activateManaFor, resolveManaAbilityRef* |
| Restricted pool and payment provenance | `rules/stack.go`: paymentDescriptor, manaAvailableFor, payManaDescriptorForSpent |
| Pool solver | `rules/mana.go`: resolveManaWith, manaPayment |
| Submit and event boundary | `rules/engine.go`: ask, Submit, decisionMadeText |
| Copying | `rules/clone.go`, `events/log.go`, `view/view.go`, `host/action.go`, `host/humanseat.go` |
| Replay and recovery | `replay/`, `host/viewat.go`, `host/undo.go`, `host/feedback.go`, `internal/testutil/feedback/` |
| Browser submission | `web/src/lib/seatpanel.svelte.ts`, `web/src/lib/api.ts`, `web/src/components/SeatPanel.svelte` |

The ordinary offer gate prices casts against the current pool. The potential
action walk uses hypothetical mana, but its aggregate bound is not an executable
payment plan: it can lose source exclusivity and activation details. Neither
AvailableMana nor PotentialActions is sufficient evidence that a cast can be
paid by a particular set of sources.

Appending newly funded casts to Decision.Options would change legacy bot
selection and recorded indices even if no client selected a plan. Therefore:

1. Preserve Decision.Options, its ordering, and every legacy Index exactly.
2. Add Decision.PaymentActions, a separate optional extension of the SAME
   priority decision, with one wrapper per supported cast and a nested plan list.
3. A wrapper references BaseOptionIndex when the same cast already exists in
   Options. Otherwise it is selectable only using the new payment selector.
4. The new UI groups a wrapper with its matching ordinary cast; it never shows
   two identical cast buttons just because both forms exist.
5. Existing seats that read only Options continue to receive the same choices.
   Production bot policies are not changed to consume the extension in this work.

Generate the extension only for a real pending priority decision. Inspecting it
must emit no event, consume no RNG, mutate no game field, or change Seq. No client
request may change which legacy options the pending decision contains.

## 3. Required first-release scope

### 3.1 Casts

V1 supports ordinary casts from the acting player's hand, on the current main
face, with a known fixed mana cost containing generic, W/U/B/R/G and true
colorless requirements. It supports ordinary fixed cost raises/reductions by
using the engine's composed cost, not printed mana value. Targeted spells are
included when the cost is independent of target choice; the player still
chooses targets normally before mana activation.

V1 does not suggest plans for X, hybrid, Phyrexian or snow costs; additional
non-mana costs; kicker or other optional/alternative casting methods; alternate
faces; casts from graveyard/exile/command; target-dependent pricing; convoke,
improvise, delve or similar contributions; or mana-spent-sensitive spell riders
such as converge, sunburst, or a bonus tied to mana provenance. These remain
manual. Detection is semantic and conservative, not a list of card names.

Do not accidentally automate an optional cost or omit a mandatory cost because
the base printed mana cost belongs to the supported subset. If a normal cast
itself is supported but an alternative method is not, only offer a plan for the
normal method, provided the existing cast flow can commit that method explicitly.

### 3.2 Mana

Required producers are untapped, controlled battlefield sources with a legal
payment-window ability whose cost is only a tap and whose production is fixed
or a finite explicit choice of colors. Cover basics, intrinsic dual-land
abilities, printed color-choice tap abilities, simple rocks/dorks, and fixed
multi-mana production (including surplus left floating). Ordinary floating
unrestricted mana is usable before activating more sources. Apply summoning
sickness, haste, phasing, CantActivate and payment-window timing restrictions
through the existing legality helpers.

The same permanent may be used only once even when it has several abilities.
The plan records the exact ability and selected production. `Add R G` produces
both; `Add R or G` produces one choice. Do not infer choices from labels.

V1 excludes activation costs involving life, sacrifice, discard, counters,
mana, untapping or other objects; converters/repeatable mana loops; foreign or
granted abilities; dynamic/reflected production; hand/graveyard mana abilities;
and restricted or special-effect production. A plain-looking land under a mana
replacement or a relevant activation/tap/mana trigger is not automatically a
simple producer. Exclude affected plans unless the implementation proves their
actual behavior fits the supported, choice-free production model.

The pool solver must preserve snow/typed/persistent/restricted provenance. For
V1 it is acceptable to decline planning for a payment whose available pool or
sources carry provenance the planner cannot model exactly. It is never acceptable
to strip the metadata and spend the mana as ordinary mana. Do not broaden the
existing restriction grammar as part of this feature.

An unsupported producer elsewhere on the battlefield does not by itself block
a plan made entirely from eligible sources. Conversely, an unmodelled global
effect that can alter a selected activation must cause fallback. Eligibility
and fallback diagnostics must explain which of these cases occurred.

### 3.3 Deferred work

Activated non-mana abilities, ward/unless/upkeep/combat payments, multiple
suggested plans, explicit source-lock preferences, strategic search over plans,
automatic hybrid/life elections, and mtg-kernel/XMage adapters are follow-ups.
Do not add debt rows to AGENTS.md; report any further exclusions in the ticket
report and commit message under the repository's closing-register rule.

## 4. Additive protocol contract

Implement shared data-only types in `decision`, below rules in the dependency
graph. Regenerate TypeScript using the existing generator. Names below are the
contract; repository-style Go names and JSON casing are specified here.

```go
// Fields on existing types, all optional on the wire.
Decision.PaymentActions []PaymentAction  // json:"payment_actions,omitempty"
Intent.Payment          *PaymentSelection // json:"payment,omitempty"
Decision.PaymentFallback *PaymentFallback // json:"payment_fallback,omitempty"

type PaymentAction struct {
    ID              string         // json:"id"
    Cast            PlannedCast    // json:"cast"
    BaseOptionIndex *int           // json:"base_option_index,omitempty"
    Label           string         // json:"label"
    Plans           []PaymentPlan  // json:"plans"
}

type PlannedCast struct {
    Object state.ObjID // json:"object"
    Face   int         // json:"face"
    Origin string      // json:"origin"; "hand" in V1
}

type PaymentSelection struct {
    ActionID string      // json:"action_id"
    Plan     PaymentPlan // json:"plan"; exact selected offered witness
}
```

PaymentPlan must have `version` (1), `id`, `cost` (structured resolved mana
requirement), ordered `activations`, `pool_spend`, and `pool_after`. Each
activation contains `source`, `source_zone_seq`, `ability`, and `produces`.
`ability` is a stable printed/intrinsic identity, not an index into a filtered
temporary list; define a discriminated identity for printed versus intrinsic
abilities and pin its codec. `source_zone_seq` identifies the latest zone entry
event for that incarnation (use the existing equivalent if one exists; define
the genesis sentinel). Production and pool quantities use nonnegative integers
and the fixed six-symbol order W/U/B/R/G/C. Cost stores generic separately from C.
Amounts must fit the existing engine types, with checked addition/multiplication.

Display labels may describe cards but never authorize an action or influence
identity/ranking. Internally the admitted action must resolve to the same
ordinary cast descriptor beginCast uses; the client supplies no script or SVar.

A manual submission is unchanged:

```json
{"seq":123,"player":0,"choices":[7]}
```

A planned submission uses an exclusive alternative selector:

```text
{seq:123, player:0, choices:[], payment:{action_id:<offered ID>, plan:<offered plan>}}
```

`choices` must be empty and `rest` absent/empty when Payment is present. The
new selector is valid only for priority. Decision.Validate must recognize this
case before applying the legacy Min/Max count to Choices. It validates the
actor, Seq, selector exclusivity, membership in PaymentActions, plan version,
ID and exact witness equality. Everything else continues through legacy
validation. In particular, an empty Choices without Payment remains subject
to today's normal bounds. A planned action without a plan cannot be selected.

Use domain-separated SHA-256 IDs over a documented canonical, map-free encoding
of version, decision Seq, acting player, cast identity and (for a plan) its
complete execution witness. Exclude labels, BaseOptionIndex, preferred rank and
the ID field itself. Use full lowercase hex digests. A change of list order or
display name cannot change a plan's identity. Neither IDs nor client-supplied
amounts are proof of legality; rules validates the witness independently.

The first implementation publishes only V1. Keep V1 ordering, codec and planner
semantics frozen once shipped; a later change that alters its recommended plan
needs a versioned successor and an explicit replay-compatibility decision. A
replay never means "select element zero from the current planner". Plan bytes
travel with the recorded intent, and unknown versions are rejected.

Reject oversized lists, duplicate/reused sources, unknown ability variants,
negative/overflowing quantities, forged production, a mismatched cast, a stale
incarnation, and any extra mutable payload outside the declared contract. Check
size before expensive hashing or search. Set named V1 constants: at most 64
activation steps and 65,536 planner search nodes per cast. Hitting a supported
resource bound is a planning limit, not a declaration that the spell is illegal.

## 5. Planner and offer construction

Introduce a pure rules-side API along these lines (internal types are allowed):

```text
planCastPayment(player, exactCast, version) ->
    {ready(plan), unsupported(reason), insufficient, search_limit}
validateCastPayment(player, exactCast, witness) -> error
```

One rules-owned implementation computes the effective cost and collects eligible
source alternatives. Reuse/refactor the existing feasibility and pool solver;
do not create a second approximation of cost modifiers or mana restrictions in
view, host or the browser. Existing membership helpers need audit: the current
windowManaUnits deliberately omits some production shapes and unlessManaReachable
returns only a bool. Returning its successful branch is a starting point, not
automatic proof that the new plan is executable.

The search operates over exclusive source alternatives and evaluates complete
pool payment with the existing solver. It must backtrack when assigning a dual
land to one color would strand a later pip. Do not greedily satisfy pips and
assume the remainder is payable. Do not enumerate permutations of equivalent
activation sequences; canonicalize independent activations by source/ability
identity. Search complexity is bounded by counters, never elapsed time.

Rank valid plans by this deterministic lexicographic preference:

1. Fewest newly activated sources (a sufficient floating pool gives zero).
2. Fewest creature sources activated.
3. Least surplus newly produced mana after paying the cost.
4. Least flexibility consumed (sum of distinct producible mana types across
   the chosen sources' eligible abilities).
5. Lexicographically smallest canonical activation witness.

Use no hidden opponent information or RNG. This is a bounded heuristic, not a
claim of optimal play. If the node limit is reached after a complete plan was
found, returning the best complete plan found is allowed and must be deterministic;
record the limit in diagnostic counters. Without a complete plan, return
search_limit. Never return a partially funded action.

Build candidate cast identities using the existing legal-action walk, retaining
timing, origin, cast prohibitions and mandatory-target feasibility checks.
Refactor a discovery/pricing callback or reuse its hypothetical discovery path
as a candidate superset, then require an exact complete plan for admission.
Do not create a second hand-only cast-legality implementation. The hypothetical
walk alone cannot admit PaymentActions.

PaymentActions is deterministic, grouped by exact cast identity, with zero or
one plan per action in V1. If the ordinary cast already exists, set BaseOptionIndex;
if not, leave it absent. The extension remains within the same decision Seq.
Cache read-only results at the decision boundary if needed, without changing
events, RNG, observers or any game's rules state. Cache keys must include the
exact decision/position and be invalidated across clone, rewind and continuation.

## 6. Validation and execution

At Submit, validate the full selection before appending an intent or consuming
the pending decision. Membership validation is followed by rules validation of
the current cast legality, source incarnations, abilities, production, cost and
pool witness. Failure leaves pending decision, events, head, state, RNG and
intent count unchanged. Host submission must perform the applicable validation
before acknowledging an invalid plan; malformed user input must not become a
match crash when the match goroutine later calls Submit. Reuse the host's existing
locking/ownership discipline; an HTTP reader never runs the engine.

Store an immutable copy of the selected witness on the cast continuation, then
enter the ordinary cast path. Do not pre-tap at priority or create a parallel
spell-resolution implementation. Targets and other normal announcements still
happen where they do today. No synthetic public mana-choice intents are required
for the deterministic steps that the selected plan already authorizes.

At the normal mana-activation stage, revalidate the whole remaining plan and
resolved cost before the first activation. Activate through the existing mana
ability path, honoring production choice without a redundant color prompt when
the witness already specifies it. All taps, mana production/spend, source
attribution and later cast consequences use the existing events/handlers.
Do not implement execution by directly writing Tapped, Pool or Life, or by
emitting a tap plus nominal mana in place of executing the ability.

After each activation, check actual production and outstanding continuation.
Do not continue blindly if it posed a replacement/other decision or changed a
remaining source. Preserve the normal rules timing for triggers, priority and
state-based actions; automatic execution grants no extra priority window.

If final cost/source validation fails before activation, retain the normal cast
continuation and expose its existing manual payment choices, with
PaymentFallback `{plan_id, reason}`. Required reason vocabulary:
`cost_changed`, `source_changed`, `production_changed`, `choice_required`.
An already illegal cast takes the engine's existing illegal-cast reversal path.
Do not advertise that a manual window can recover an illegal cast.

If a real activation unexpectedly interrupts a plan, suspend through the normal
decision machinery. Cancel automatic execution of its remaining steps and
continue manually when that interruption resolves. Preserve completed legal
activations and floating mana under the existing transaction/reversal rules;
do not roll them back using an ad hoc snapshot. Never substitute another source,
silently add activations, or spend unannounced life/sacrifices. Version-one
eligibility should make this exceptional, but the executor must handle it safely.

## 7. Replay, logging and privacy

The selected witness is part of the existing persisted Intent. Add a canonical
payment suffix to DecisionMade.Text ONLY for planned submissions, incorporating
the action and plan IDs (which bind the witness). Freeze that encoding with a
golden. Manual decisionMadeText and existing events.Kind ordinals stay untouched.
Ordinary mana/cast events still describe the actual effects. A receipt UI may
derive execution from those events; the plan preview is not proof of execution.

Normal replay reconstructs the same priority decision and validates the recorded
V1 witness. Exercise Replay, ReplayTo, Engine.Clone at a target ask with a pending
plan, host restart, undo and feedback/repro. No original client setting is needed.
Reconstruction must not depend on an in-memory offer cache that existed only in
the live process. A change of the client's toggle requires no replay metadata.

Audit all copies of Decisions, Options, Intents and pending casts. Deep-copy new
mutable slices at ownership boundaries (host pending, view projection, human
parking, engine cloning and persisted intent admission). Caller mutation after
submission or projection must not alter either the live plan or its logged copy.
Do not apply a shared-cache pointer to two engines after cloning.

PaymentActions and pending PaymentFallback are visible only to the acting seat
where its ordinary decision is visible. Their cast identities can expose its
hand. Other seats/public spectators must not receive them, including through
events, error text or diagnostics. Follow existing omniscient-view policy.
Plan execution reveals only what the ordinary activation/cast flow reveals.
Hashing a hidden plan is not a replacement for redaction.

## 8. Browser and external-agent behavior

Add `Auto-pay mana` to the seated player's controls, initially off. Keep the
preference local to that seat's match view; it persists across decisions/turns
in that view and resets to off on a different match/seat. Reload persistence is
not required for V1. Spectators have no control. It must be possible to switch
on/off repeatedly without posting any intent.

With the preference on, a normal click on a cast with a plan submits its first
plan. Show a source summary before submission (existing action menu/tooltip is
acceptable). With it off, ordinary casts take the legacy route and plan-only
casts remain visibly conditional on using a plan rather than sending an invalid
legacy index. Provide an explicit `Cast with suggested mana` action to use a
plan for one cast while the preference is off, and `Pay manually` when an ordinary
cast option exists while the preference is on. If no ordinary cast is offered
yet, the manual route is the existing mana-tapping controls followed by casting.

Keep one card/action group for a cast even when it has both a legacy option and
a PaymentAction. The UI honors Plans order and supports rendering multiple
entries in a fixture, though the engine produces one in V1. Use IDs for selection,
not row numbers or labels. Do not post all listed plans.

Toggling after a submission affects future submissions only. Double-clicks,
in-flight requests, refreshed decisions and stale HTTP responses must not attach
an old plan to a new Seq. Reject/adopt a fresh decision using existing stale-intent
handling. Display PaymentFallback and the real pending manual ask when necessary.

Audit auto-pass and hold-priority behavior: when plan use is enabled, a playable
planned-only cast must prevent an "empty window" auto-pass. With the preference
off, preserve the current manual auto-pass policy. Merely toggling must not
auto-submit a cast or accidentally release an explicitly held priority window.

An external agent receives the same extension and submits the same Payment
selector. No special engine object, privileged planner call or game-wide mode is
needed. A small test seat that deliberately chooses the first plan demonstrates
this; changing the production bot's strategy is out of scope.

## 9. Acceptance criteria

Every row requires an automated assertion at the narrowest useful layer. Each
implementer reports test names/commands and maps them to these IDs. Shared setup
must exercise the real offer/Submit/activation/payment paths where specified.

| ID | Required assertion |
|---|---|
| PP-01 | With an empty pool and Island/Swamp/Mountain, a supported `{1}{U}{B}` cast appears in PaymentActions; its source plan is complete. Options and its indices remain the legacy list. |
| PP-02 | A cast also payable from floating mana groups with its ordinary option via BaseOptionIndex; it is not duplicated in the UI. Sufficient pool yields zero activations. |
| PP-03 | Mountain is preferred to Badlands in the opening example; the latter remains untapped after the selected plan executes. Fixed order and repeated runs yield identical IDs/witnesses. |
| PP-04 | A dual source is not counted twice. A cast requiring two mana is withheld when only one single-output dual exists. A case requiring backtracking finds a valid complete assignment. |
| PP-05 | True colorless and generic remain distinct. A colored source cannot pay `{C}`; it can pay generic. Fixed multi-output production leaves the correct surplus. |
| PP-06 | Simple dork/rock production works; tapped, sick-without-haste, phased, wrong-controller and activation-prohibited sources are excluded. A sick source with haste is admitted when otherwise legal. |
| PP-07 | Forbidden timing, absent mandatory targets and CantBeCast continue to suppress planned casts even with abundant mana. The engine computes the effective fixed taxed/reduced cost. |
| PP-08 | X, hybrid/life/snow, additional/alternative costs, target-dependent pricing and mana-sensitive riders receive no V1 plan. Their legacy offers/asks remain unchanged. |
| PP-09 | Costly/restricted/dynamic producers and relevant unmodelled global mana/tap effects cannot fund a nominally simple plan. An irrelevant unsupported producer does not suppress a valid basic-land plan. No restricted pool metadata is erased. |
| PP-10 | Inspecting/reinspecting offers changes no log, RNG, state or Seq. Node/size limits terminate deterministically; no partial plan is offered. Diagnostics distinguish unsupported, insufficient and search-limit outcomes. |
| PP-11 | Planned casts require empty Choices/Rest, the proper actor/Seq/kind, and an offered action/plan. Unknown version, ID, ability, forged quantities, duplicate source, changed incarnation and mixed selectors reject before mutation. Legacy validation still holds. |
| PP-12 | The real Submit flow asks for targets before activating the selected mana sources. The ordinary target answer resumes the stored plan without extra tap/color asks for a supported source. |
| PP-13 | The exact listed sources/abilities/colors execute; ordinary mana activation markers, production, spend, cast provenance and final pool are correct. No extra priority pass or stack object is introduced by payment. |
| PP-14 | A controlled post-offer cost/source change invalidates execution before tapping and opens the appropriate manual/reversal path. An injected activation interruption cancels remaining automation, preserves completed legal effects, and never substitutes sources. |
| PP-15 | Plan payload survives JSON intent persistence, Replay and ReplayTo, cloning at a target ask, host restart and undo. Tampering with the saved witness rejects/diverges. Plan selection affects the chain-bound DecisionMade encoding; all manual encodings/goldens remain identical. |
| PP-16 | Mutating returned offer slices, a submitted witness or a clone cannot change the live game, another view or recorded intent. A new Seq never accepts a previous offer. |
| PP-17 | Opponents/public spectators cannot see PaymentActions or fallback details. HTTP table/seat credential fencing holds for the new selector, and invalid submissions do not crash a match. |
| PP-18 | UI toggles on/off/on during one turn without issuing intents. Subsequent clicks include/omit Payment appropriately; the explicit one-cast action works while off. Pending submitted plans are unaffected by later toggling. |
| PP-19 | UI groups base/planned casts, renders a multi-plan fixture by stable IDs, handles stale responses and double submissions, shows fallback, and does not auto-pass an enabled planned-only cast. |
| PP-20 | A test Seat chooses the extension through the normal host API and completes a deterministic replayable game. Another Seat ignores the extension and retains the legacy action/intent behavior. |
| PP-21 | The corpus is present and existing TestHeads, repo-deck ratchets and focused conformance gates pass without golden updates. Fixed-seed hosted planned games finish without new errors/livelocks and replay identically. |

Tests must use authored minimal IR fixtures or the gitignored corpus, never
committed Forge script text. Test helpers must not bypass the very validation
or execution being asserted. Any unexpected red committed baseline must be
investigated under AGENTS.md; do not fix another session's uncommitted files.

## 10. Verification and release evidence

Use focused tests for each slice. At integration, run the repository's required
gates, plus tests covering all PP rows. Suggested gate commands (adapt package
selection to actual changes, preserving the coverage intent):

```sh
go test ./decision ./events ./rules ./view ./seat ./replay ./host ./host/httpapi ./cmd/repro -run 'Test.*PaymentPlan' -count=1
go test ./rules -run 'TestHeads|TestEveryRepoDeck|TestRepoDeckGamesReplayExactly' -count=1
make conformance
make sim
go run ./cmd/gentypes -check
git diff --check
```

Use the web package's actual package.json scripts for its focused tests, type
checks and lint. Names of the new tests should contain PaymentPlan so the first
command executes them; identify all selected tests and verify none were skipped
for a missing corpus. `make sim` is a legacy-regression gate, not evidence that
a planned cast was exercised: add the explicit plan-selecting test-seat lane.

Measure offer-generation cost over a fixed set of ordinary and broad-source
boards, search nodes, complete-plan hit rate, unsupported/limit reasons, external
decision counts and finished replayable games. Compare the same snapshots with
and without the extension for offer overhead; compare a manual execution of the
same selected payment with planned execution for state equivalence. Do not claim
a policy win-rate or mtg-kernel speedup from this feature's smoke evidence.

Release report must contain the PP-to-test matrix, commands/results, corpus pin,
source commit, supported/excluded shapes, deterministic search limits, observed
offer cost/fallback counts and the exact manual golden check. No production-bot
promotion or change of planner ranking is hidden in the release.

## 11. Queue breakdown and landing order

Companion briefs live in `docs/superpowers/plans/cast-payment-plans/`. Each is
written for direct `agentctl issue add --brief-file` intake, not a triage task.
Use these IDs and dependencies (or rename all consistently before intake):

| Ticket | Deliverable | Depends on |
|---|---|---|
| payplan-01-contract | Data types, canonical identity, validation and copy boundaries | none |
| payplan-02-planner | Pure bounded planner and dormant offer builder | payplan-01-contract |
| payplan-03-execution | Submit, continuation, event commitment and replay | payplan-02-planner |
| payplan-04-host | Publish supported offers, host admission, projection, restart/undo/feedback | payplan-03-execution |
| payplan-05-client | Toggle, grouped actions, selection, auto-pass and fallback UX | payplan-04-host |
| payplan-06-acceptance | External-seat exercise, regression gates, coverage/performance report | payplan-05-client |

Keep the offer builder dormant on normal engine/host decisions until submission
and replay can consume it. Ticket 03 can exercise the builder via focused tests;
ticket 04 enables publication in live pending decisions with all transport and
privacy paths present. No landed stage may advertise an unexecutable action.

Each ticket makes its changes in its own agent-worktree.sh worktree, rebases on
its merged dependencies, stages explicit paths, passes relevant gates, lands,
and removes its worktree. Do not dispatch shared-source edits concurrently.
No ticket may grow the AGENTS.md approximation register, replace the existing
payment engine, commit corpus scripts, or bind demo ports 8080/8081.

The expected implementation effort for this bounded release is approximately
8-12 engineer-days, not a deadline or an agent runtime budget. Planning and
continuation correctness dominate; multiple plans and more payment shapes require
their own acceptance criteria. A ticket is complete only when its assigned
criteria pass; the feature is complete after payplan-06 closes the full matrix.

## References

* mtg-kernel PaymentPlan and backtracking solver, inspected commit `5472539`:
  https://github.com/adams-shaun/mtg-kernel/blob/54725398f9fac767c8ee2cef1be6b36cd3656ca2/mtg-kernel/src/mana.rs
* XMage separates pool spending and mana activation; its human path has special
  care for spells whose result depends on spent colors, inspected commit `7bbfb31`:
  https://github.com/magefree/mage/blob/7bbfb31587b33f070d5d5efd153ef0d0217afffa/Mage.Server.Plugins/Mage.Player.Human/src/mage/player/human/HumanPlayer.java
* Gorge's earlier experiment motivates considering complete plans rather than
  searching isolated taps: `docs/superpowers/reports/2026-09-24-pn22-mana-tap-search.md`.

These are design references. Do not port their runtime dependencies or bypass
gorge's event and privacy boundaries to reproduce their implementation details.

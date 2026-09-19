# Botbench decision-trace design

## Purpose

Before changing another bot heuristic, make its decisions inspectable and
comparable without changing a game.  `cmd/botbench` will gain an opt-in,
deterministic JSON Lines trace that records the information available to the
seat making each decision, its legal action shapes, and the action it chose.
The trace is an offline diagnostic artifact.  It is not an event, replay
input, policy input, or a learned model.

The immediate goal is to identify high-frequency, high-consequence decision
families on a small, stable deck suite.  It does **not** change `botpolicy`.

## Scope and non-goals

In scope:

- an explicit `botbench` trace file flag and a versioned JSONL contract;
- deterministic collection when games run concurrently;
- a privacy-safe seat-perspective feature projection;
- a fixed initial five-deck, two-player evaluation suite and seed split;
- aggregate diagnostic proxies derived from the trace; and
- regression tests for determinism, redaction, and replay isolation.

Out of scope:

- modifying a bot decision rule;
- recording Forge scripts, raw `view.View`, engine continuation state, or
  opponent private zones;
- training or evaluating a runtime learned model;
- writing trace data into events, replays, chain heads, or goldens; and
- claiming a counterfactual "regret" score before an evaluator exists.

## Command interface and artifact lifecycle

`cmd/botbench` receives an opt-in flag:

```
-decision-trace <path>
```

An empty flag value leaves the current run, output, allocations in the
decision loop, and results unchanged.  A non-empty value is a destination
file, not a directory.  The parent directory must already exist and the
destination must not exist; either condition is an error.  This prevents a
bench command from unexpectedly creating a tree or overwriting a previous
analysis.

The command writes to a temporary sibling file and renames it to `<path>`
only after every game succeeds and every record is validated.  A failed run
returns its original error and removes the temporary file.  It never leaves a
partial file at the requested destination or silently disables tracing.

Trace files are local analysis artifacts, normally written outside the
repository (for example under `/tmp`).  They are not committed: even though
the initial suite uses public test decks, a future suite can include private
decision context and trace size is not source history.  Checked-in tests use
small hand-built expected records and redaction assertions, never a captured
full match trace.

## Ordering and integration

`botbench` already runs games in workers and folds game slots in stable order.
The trace follows that same boundary:

1. A game collects its own decision records in decision-sequence order while
   it runs.  It receives its outcome only when that game ends.
2. Workers return these per-game buffers with their existing game result;
   workers do not write files and cannot determine output order.
3. After all workers finish with no error, the caller emits one run header,
   then every game buffer ordered by pair index, game index, and decision
   sequence, followed by that game's terminal record.

This makes byte output independent of worker count, completion order, map
iteration, and wall clock.  Trace collection is observational: it runs after
`Seat.Decide` and before `Engine.Submit`, reads no policy RNG, and never calls
into the policy again.  The existing `decisionStats` and `actionCoverage`
collectors remain independent, opt-in observers.

## JSONL schema

Every line is one JSON object with `record_type` and `schema_version`.  A
v1 reader must reject an unknown schema version rather than guessing its
meaning.  Fields added later receive a new schema version when their meaning
would alter existing analysis.

### Run record

The first line is `run-v1`.  It identifies the invocation sufficiently for a
reproducible comparison:

- trace and board schema versions;
- policy names, base seed, games per pair, seats, format, watchdog values and
  pair manifest in its stable execution order;
- trace split (`development` or `heldout`); and
- the named suite when the run matches the initial `mono5` suite.

The header records the actual resolved pairs rather than trusting an analysis
tool to reconstruct a flag string.

### Decision record

Each `decision-v1` record has a pair index/name, game index, game seed,
decision sequence, deciding seat and policy, plus:

- decision kind, minimum and maximum choice counts;
- a list of legal option shapes, in engine-offered order;
- the chosen option indices, in submitted order; and
- a `board-v1` feature snapshot from the deciding seat's perspective.

An option shape includes only the wire data a policy analysis needs: index,
kind, object/player/attacker IDs, required flag, exclusivity group,
alternative-cost index, cast mode, amount, ability index, and the existing
structured target-effect summary.  It deliberately omits `Label`, `Prompt`,
`Source`, every `Resume*` field, roll values, and compiled SAs.  Object IDs
are scoped to one game and exist only to join an offered option to a feature
in that same record.

`board-v1` is a JSON-safe projection of `botpolicy.Board`, built while the
current game-shaped Board is valid.  It is copied into sorted slices, never
serialized from Go maps.  Its fields are:

- `is_main` and the deciding seat's fixed-order mana pool;
- that seat's own `Card` feature facts (no card name or script text);
- public life totals;
- public creature facts (controller, P/T, damage, tapped and keywords);
- public commander cast/zone/damage facts; and
- public stack entries in stack order.

The snapshot contains no opponent hand, library, graveyard, mana pool, raw
private choice, hidden card identity, or server continuation state.  The
deciding seat's own card and mana features are intentionally present: they are
the data the production bot may see and use.  Consumers must treat each row
as one seat's perspective and must never merge another seat's private feature
rows into an input feature vector.

### Terminal game record

One `game-v1` record follows each game buffer.  It contains pair/game/seed
identity, winner seat or draw, stall category, turn count, intent count,
starting seat when present, and a livelock diagnostic only for a livelock.
It has no game state snapshot.

## Initial evaluation suite

The first suite is exactly these existing 60-card fixtures, one per colour:

| Colour | Fixture | Primary decision pressure |
| --- | --- | --- |
| White | `mono-white-equipment` | equipment abilities and targets |
| Blue | `mono-blue-tempo` | counter timing and reactive choices |
| Black | `mono-black-aggro` | removal targets and combat |
| Red | `mono-red-prowess` | mana/cast ordering and spell targets |
| Green | `mono-green-stompy` | mana development and creature combat |

Red Prowess is intentionally selected over Mono-Red Goblins: its spell-order
and targeting decisions exercise the first planned heuristic families more
directly.  The suite runs all ten unordered two-deck pairs.  The header names
the exact pair order, so future suite growth cannot silently alter a result.

Two disjoint, documented samples prevent a heuristic from being selected on
the same seeds that certify it:

- development: all ten pairs, 100 games per pair, `base seed 0`;
- heldout: all ten pairs, 400 games per pair, `base seed 1,000,000`.

The development run diagnoses and iterates.  Only the heldout run supports a
claim that a proposed policy beats its predecessor.  Both use seat trading,
same-policy controls, and the existing watchdogs.  Any reported comparison
also states stalled/error counts, starting-player split, pair-level intervals,
and replay status; a positive pooled rate alone is not sufficient.

## Diagnostic report

A trace analysis report groups records by decision family and policy.  Its
v1 measures are descriptive proxies:

- opportunities and offered-option counts;
- selected option-kind, cast-mode, and selected-position distributions;
- singleton share and option-set width;
- terminal outcome, turns and stall association; and
- breakdown by deck pair and starting player.

It may identify flat or frequently forced policy branches, but it must call
these signals *diagnostic proxies*, not regret or proof that another offered
choice would have won.  A counterfactual score requires a later, separately
validated evaluator.

## Failure handling and tests

Implementation tests must establish all of the following:

- default runs are byte-identical to pre-trace output and do not construct a
  trace collector;
- trace bytes are identical for equivalent runs with one worker and multiple
  workers, including matrix pair order and per-game decision ordering;
- a trace-enabled game has the same submitted intents, event stream, head,
  and verified replay as the equivalent trace-disabled game;
- records contain all and only the declared option/board fields, with explicit
  redaction fixtures for every opponent private zone and every engine-only
  continuation field;
- the snapshot is stable after the reusable `botpolicy.Board` has been
  refilled for a later decision;
- existing destination, absent parent, write failure, validation failure, and
  rename failure leave no completed destination; and
- the run header captures the actual suite, pair manifest and split.

Only after these tests and the initial development/heldout baselines exist may
the next design choose one policy family (likely mana/cast sequencing, target
valuation, or combat) for a behaviour change.

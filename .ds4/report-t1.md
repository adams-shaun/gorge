# Report — fb-20260922T145544Z

Restricted floating mana now projects onto the pool readout, so a seat that
holds mana the engine will not spend on the cast they are looking at can see
why.

Commit: `19a2b712 feat(view): project restricted mana onto the pool readout`

- `.cards` in this worktree was already a symlink to
  `/home/sadams/projects/gorge/.cards` (found present, not created).
- `.ds4/` was copied by the controller; `issue.md`, `ledger.json` and prior
  reports were present.

## What changed, per file

- **`view/view.go`**
  - New `PlayerView.PoolRestrictions []PoolRestrictionView` field,
    `json:"pool_restrictions,omitempty"`, inserted after `Pool`, with the
    public-information CR 106.4a/106.4b rationale and the byte-identity
    contract (omitempty, the `Available` convention).
  - New `PoolRestrictionView{Color string; Amount int32; Text string}` (json
    `color`, `amount`, `text`).
  - Projection at the existing site next to `pv.Pool = poolView(p.Pool)`:
    `pv.PoolRestrictions = poolRestrictions(g, p.RestrictedMana)` — filled for
    every seat under every visibility, exactly like `Pool`.
- **`view/poolrestriction.go`** (new) — `poolRestrictions` projects each batch
  (skipping `Valid == ""` batches, which impose no spend limit), and
  `humanizeRestrictValid`/`humanizeRestrictTerm`/`humanizeSpec` render the
  common shapes: `Spell.<spec>` → "spend only to cast <spec>", `Activated.<spec>`
  → "spend only to activate <spec>", bare `Spell`/`Activated`/`nonSpell`, with
  card types/supertypes/colours recognised. `ChosenType` resolves against
  `g.Obj(batch.Source).ChosenType`. Anything unrecognised (e.g. `MultiColor`,
  `wasCastFromYourHand`) makes the WHOLE Valid$ fall back to the raw string
  rather than a half-prosed mixture. Multi-term Valid$ (OR semantics) joins
  with " or ". `view/` does not import `rules/`.
- **`view/pool_restriction_test.go`** (new) — the projection pins (see below).
- **`view/projection_closure_test.go`** — added `pool_restrictions: true` to
  the `PlayerView` JSON-key allowlist with the public-fact rationale. This is
  the D6 god-view gate firing correctly on a new key; the field is public, like
  `available` and `pool`, so it belongs in the allowlist, not behind a gate
  exemption.
- **`web/src/components/ManaPool.svelte`** — new optional
  `poolRestrictions` prop, rendered as persistent text (`data-mana-restrictions`,
  `data-mana-restriction="<sym>"`) inside the pool group, beside the chips, per
  the component's own B1 persistent-text contract. Outer and pool-group
  conditions extended to draw the annotation even when the pool map is empty.
  Absent/empty/null draws exactly the old markup.
- **`web/src/components/{IdentityBar,Rail,SeatPanel}.svelte`** — thread
  `player.pool_restrictions` / `focused.pool_restrictions` / `mine.pool_restrictions`.
- **`web/src/components/ManaPool.svelte.test.ts`** — rendering pins (below).
- **`web/src/protocol.ts`** — regenerated via `go run ./cmd/gentypes` (not
  hand-edited); `PoolRestrictionView` and `pool_restrictions?` appear.

Engine (`rules/`), `state/`, `events/`, heads and goldens are untouched.

## Required tests added

- `TestRestrictedManaIsProjected` (`view/`) — the replayed snapshot shape: one
  `{B}` batch `Valid:Spell.Creature+ChosenType` with source `ChosenType:"Demon"`
  plus one bare `{B}` (the control). Asserts preconditions first: the batches
  really sit in `g.Players[0].RestrictedMana`, and the produced text really
  differs from the raw Valid$ fallback (so a formatter that fell through
  fails). Expects "spend only to cast a Demon creature spell" for the
  restricted batch, nothing for the bare one, and a second `Spell.Creature`
  seat expecting "spend only to cast a creature spell".
- `TestRestrictedManaExoticValidFallsBackRaw` — raw fallback floor and the
  `Spell.Instant,Spell.Sorcery` "or" join.
- `TestPoolRestrictionsOmittedWhenEmpty` — a seat with no restricted mana
  serialises no `pool_restrictions` key and carries a nil slice.
- `TestPoolRestrictionIsPublicForEveryViewer` — projected for every seat under
  Seat/Public/Omniscient and every viewer, like `Pool`.
- `ManaPool.svelte.test.ts` — annotation is persistent markup keyed by colour;
  one annotation per batch not per symbol; absent/null/empty renders
  **byte-identically** to the pre-change output; a blank-text entry draws
  nothing.

## Gates run (real output)

Required view command:

```
$ go test -run 'TestRestrictedManaIsProjected|TestCR106ManaPoolIsPublicForEveryPlayer' ./view/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/view	(cached)
```

Full `view` package (I edited it; run once):

```
$ go test ./view/
ok  	github.com/adams-shaun/gorge/view	0.807s
```

Web rendering pin:

```
$ cd web && npm test -- src/components/ManaPool.svelte.test.ts
 Test Files  1 passed (1)
      Tests  18 passed (18)
```

Wire types:

```
$ go run ./cmd/gentypes && go run ./cmd/gentypes -check
OK
```

Format:

```
$ gofmt -l view/*.go
(no output)
```

Behaviour goldens outside `rules/`:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.155s
```

Extra (not required by the brief; done because I threaded three components):
`cd web && npm run check` → `0 errors and 1 warning` (the warning is a
pre-existing `ResolvedCard.svelte` state-capture warning, unrelated).

## Fails without the fix

Each new test was shown to fail with the non-test change reverted; the file was
then restored and `cmp`-verified byte-identical.

(1) Projection call removed from `view/view.go` (`pv.PoolRestrictions = …`
deleted):

```
--- FAIL: TestRestrictedManaIsProjected (0.00s)
    pool_restriction_test.go:83: seat 0 projects 0 pool restrictions, want 1 (the bare {B} carries no limit): []
--- FAIL: TestRestrictedManaExoticValidFallsBackRaw (0.00s)
    pool_restriction_test.go:159: projects 0 restrictions, want 1
--- FAIL: TestPoolRestrictionIsPublicForEveryViewer (0.00s)
    pool_restriction_test.go:203: visibility seat viewer 0: seat 0 projects 0 restrictions, want 1
FAIL
FAIL	github.com/adams-shaun/gorge/view	0.002s
```

(2) Humanizer forced to return the raw Valid$ (`return valid` at the top of
`humanizeRestrictValid`):

```
--- FAIL: TestRestrictedManaIsProjected (0.00s)
    pool_restriction_test.go:90: precondition: the projection fell through to the raw Valid$ string instead of humanising it
--- FAIL: TestRestrictedManaExoticValidFallsBackRaw (0.00s)
    pool_restriction_test.go:179: two-alternative text = "Spell.Instant,Spell.Sorcery", want "spend only to cast an instant spell or cast a sorcery spell"
FAIL
FAIL	github.com/adams-shaun/gorge/view	0.002s
```

(3) Restriction-rendering block removed from `ManaPool.svelte`:

```
FAIL  src/components/ManaPool.svelte.test.ts > ManaPool > renders one annotation per restricted batch, not one per pool symbol
AssertionError: expected '...data-mana-pool="" ...' to contain 'data-mana-restriction="B"'
 Test Files  1 failed (1)
      Tests  2 failed | 16 passed (18)
```

Restoration was `cmp`-verified (`RESTORED`, `RESTORED2`,
`RESTORED-BYTE-IDENTICAL`).

## Brief premise check

The brief's corpus prevalence was re-measured and held exactly:

```
$ /usr/bin/grep -rlE 'AB\$ Mana.*RestrictValid\$' .cards/cardsfolder | wc -l
167
$ /usr/bin/grep -rhE 'AB\$ Mana.*RestrictValid\$' .cards/cardsfolder | wc -l
170
```

The feedback snapshot directory exists at the path the brief gave.

## Deviations from the brief

- **`view/projection_closure_test.go` allowlist edit.** The brief did not
  mention this file, but `TestViewMarshalsClosed` fails the build on any new
  `PlayerView` JSON key. `pool_restrictions` is a public fact (same class as
  `available`/`library_top`), so I added it to the allowlist with a rationale
  comment rather than exempting the test. This is the gate doing its job, not a
  widened condition.
- **Batches with an empty `Valid` are omitted.** The brief says "one entry per
  batch". A batch with `Valid == ""` (an `AddsNoCounter$`-only batch, e.g.
  Boseiju) carries pool provenance but no spend limit; annotating it would
  mislead. It is skipped, so `PoolRestrictions` is one entry per *restricted*
  batch. Recorded in the field doc and here.
- **Raw-string fallback is all-or-nothing per Valid$.** A mixed Valid$ where
  one alternative is exotic returns the raw whole string, not a partial
  prose/raw join. Named in the humanizer doc.
- **`ManaPool.svelte` outer condition widened** to `|| restrictions.length > 0`
  so the annotation draws if restriction state ever outlives its pool chips.
  With an empty/absent restriction list the markup is byte-identical to before.

## Issues

1. **Follow-up (ticket-worthy): annotate the any-colour decision prompt at
   production time.** The restriction is only visible AFTER the mana is
   produced. `rules/mana.go`'s `askManaColor` path poses "Choose a colour of
   mana" / Cavern's "Add one mana of any colour" with no indication that the
   result will be restricted. Annotating that prompt (engine-side decision
   text) would tell the player before they commit, but it changes decision text
   mid-chain and moves goldens, so it is out of scope here. This is the
   follow-up the brief's scope boundary names.
2. **`ChosenType` resolution is live, not LKI.** If the producing permanent has
   left the battlefield or recorded no `ChosenType` by the time the view
   projects, the text degrades to "a creature spell of the chosen type"
   (`humanizeSpec`). The restriction itself also fails closed in that case
   (rules-side), so the annotation is honest but cannot name the type. Not a
   defect in this change; noting it.
3. **No CR-lane test proposed.** This is a `view/` projection with no CR-rule
   behaviour change; the engine admission semantics are already pinned in
   `rules/paramcensus_targeting_type_choice_test.go`. No new conformance-lane
   test is warranted.

No Known-approximations row was added, grown, or closed (this is a new surface,
not a row's remainder); `knownApproximationRows` is unchanged.

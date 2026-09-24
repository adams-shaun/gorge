# Task 2835347f — `AddTrigger$ A & B` registers no grant (Mirror Shield family)

## Summary

An Equipment/Aura whose printed static grants several triggered abilities via
Forge's ` & `-joined `AddTrigger$` value granted **nothing**: `rules/layers.go`
`staticEffects` read the whole value as one SVar name, looked up a nil body and
silently emitted zero grants. Split the value through the **one exported**
`cards` grant-name splitter, emitting one grant per successfully parsed name,
and routed the existing `cards/kw_class.go` call sites through the same
exported name.

Commit: `3c015837d06b4e7f1c35de2b7b6f259ab9dc34e2`

## Per-file changes

- **`cards/kw_class.go`** — renamed unexported `splitGrantNames` to
  `SplitGrantNames` (exported) and updated the three call sites
  (`AddStaticAbility`, `AddTrigger`, `AddReplacementEffect`). ONE home for the
  ` & ` grammar; no second copy exists in `rules/`.
- **`rules/layers.go`** — the `AddTrigger$` branch of `staticEffects` now
  iterates `cards.SplitGrantNames(raw)` and emits ONE `gt` grant per name whose
  SVar body parses. Order is `SplitGrantNames`' left-to-right value order (a Go
  slice, not a map), so replay is deterministic. Each name still **fails closed
  on its own** — a missing/unparseable body grants nothing and no longer
  suppresses a valid sibling (kept deliberately).
- **`cards/splitgrantnames_test.go`** (new) — direct unit test of the exported
  splitter: `"A & B"` → `["A","B"]`, whitespace trimmed, empty members dropped,
  single name unchanged, empty/`" & "` → nil.
- **`rules/addtrigger_multiname_test.go`** (new) — three corpus-backed tests
  (registration, AttackerBlockedByCreature firing, mode-agnostic Attacks firing).
- **`rules/paramcensus_test.go`** — comment-only drift fix on the
  `checkGrantedStaticTriggers` read root (see Deviations); no read-root list
  changed.

## Why this structural shape

The grammar already had a working implementation inside `cards/` (`kw_class.go`),
used by the `K:Class:` level-grant path. Rather than re-implement the split in
`rules/`, the splitter is now exported and both callers share it — the repo's
ONE-home rule. A future `Add*$` reader that uses `cards.SplitGrantNames` cannot
miss the grammar; the next sibling is covered by construction, not by a list.

## Gates (real output)

Focused tests + splitter:

```
$ go test -run 'TestMirrorShieldMultiNameAddTriggerRegisters|TestMirrorShieldGrantDestroysDeathtouchBlocker|TestVeteransArmamentsMultiNameAddTriggerFires|TestSplitGrantNames' ./rules/ ./cards/
ok  	github.com/adams-shaun/gorge/rules	0.042s
ok  	github.com/adams-shaun/gorge/cards	0.003s
```

Structural golden:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	4.004s
```

Botbench golden (unchanged — none of the five cards is in a repo deck):

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.694s
```

Class-path regression check (I edited `cards/kw_class.go`):

```
$ go test -run 'TestClass' ./cards/
ok  	github.com/adams-shaun/gorge/cards	0.001s
```

Format / types:

```
$ gofmt -l rules/layers.go rules/addtrigger_multiname_test.go rules/paramcensus_test.go cards/kw_class.go cards/splitgrantnames_test.go
(empty — clean)
$ go run ./cmd/gentypes -check
(no output — clean)
```

Verbose confirmation that the tests actually RAN (corpus present; no skip):

```
=== RUN   TestMirrorShieldMultiNameAddTriggerRegisters
--- PASS: TestMirrorShieldMultiNameAddTriggerRegisters (0.00s)
=== RUN   TestMirrorShieldGrantDestroysDeathtouchBlocker
--- PASS: TestMirrorShieldGrantDestroysDeathtouchBlocker (0.00s)
=== RUN   TestVeteransArmamentsMultiNameAddTriggerFires
--- PASS: TestVeteransArmamentsMultiNameAddTriggerFires (0.00s)
ok  	github.com/adams-shaun/gorge/rules	0.065s
```

Worktree state: `.cards` was PRESENT (pre-existing symlink) — the corpus tests
ran, not skipped.

## Fails without the fix

`rules/layers.go` was reverted in place to the old whole-value read, the one
test command run, then the file restored byte-identically (`cmp` clean against
`.ds4/scratch/layers.go.fixed`):

```
--- FAIL: TestMirrorShieldMultiNameAddTriggerRegisters (0.00s)
    addtrigger_multiname_test.go:67: Mirror Shield live AddTrigger grants = 0, want 2 (TrigBlocks + the Secondary$ TrigBecomeBlocked); a one-name fix registers 1
--- FAIL: TestMirrorShieldGrantDestroysDeathtouchBlocker (0.00s)
    addtrigger_multiname_test.go:121: Mirror Shield granted AttackerBlockedByCreature GrantTriggerPush events = 0, want 1
--- FAIL: TestVeteransArmamentsMultiNameAddTriggerFires (0.00s)
    addtrigger_multiname_test.go:160: Veteran's Armaments live AddTrigger grants = 0, want 2 (HeroAttack + HeroBlock)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.038s
```

After restoring: `cmp rules/layers.go .ds4/scratch/layers.go.fixed` →
IDENTICAL; test green again.

## Test precondition coverage (each test can fail)

- `TestMirrorShieldMultiNameAddTriggerRegisters`: asserts the shield is on the
  battlefield, that its `AddTrigger$` value is the ` & `-joined string under
  test, that the equip actually happened, and that BOTH grants registered with
  one `Secondary$` — a one-name fix fails on the count.
- `TestMirrorShieldGrantDestroysDeathtouchBlocker`: asserts the equipped bear
  prints no become-blocked trigger (so the destroy is the grant), and the
  blocker is a live battlefield deathtouch creature (so the destroy is not
  vacuous). Reverting the fix yields 0 `GrantTriggerPush` events.
- `TestVeteransArmamentsMultiNameAddTriggerFires`: asserts the real card's
  ` & `-joined value, the equip, that the bearer prints no Attacks/Blocks
  trigger, 2 registered grants, and 2/2 → 3/3 P/T movement (different values).

## Head / ratchet / botbench movement

- **Chain heads:** not run (daemon gate). None of the five carriers is in any
  repo deck, so no golden replay involves them.
- **Ratchet:** unchanged; no `knownUnsupported`/`knownUnsupportedParams` entry
  names these cards or the primitive.
- **Botbench:** `TestConstructedDefaultIsByteIdentical` passes unchanged, as
  expected. Confirmed none of the five cards is in a repo deck:
  `Mirror Shield 0 / Veteran's Armaments 0 / Astrologian's Planisphere 0 /
  Candlekeep Sage 0 / Noble Heritage 0` (`grep -rl` over
  `internal/testutil/decks/`).

## Brief premise re-measurement (all held)

```
$ /usr/bin/grep -rlE 'AddTrigger\$[^|]*&' .cards/cardsfolder | wc -l
5
$ /usr/bin/grep -rlE 'AddSVar\$[^|]*&' .cards/cardsfolder | wc -l
23
$ /usr/bin/grep -rlE 'AddStaticAbility\$[^|]*&' .cards/cardsfolder | wc -l
3
$ /usr/bin/grep -rlE 'AddReplacementEffect\$[^|]*&' .cards/cardsfolder | wc -l
2
```

Every count in the brief held.

## Deviations from the brief

- The brief suggested the splitter unit test could live in `rules/`; it lives in
  `cards/` (`cards/splitgrantnames_test.go`) because that is the package that
  owns the exported function — the brief's `-run` pattern covers both packages.
- The optional adjacent comment-drift fix in `rules/paramcensus_test.go` was
  taken (comment-only; the declared read-root string list is unchanged).

## Issues (found, NOT fixed — for the operator)

1. **Sibling `Add*$` parameters with the same unsplit grammar**, in the same
   `staticEffects` loop in `rules/layers.go`:
   - `AddSVar$` (`rules/layers.go`, whole-value `fc.SVars[raw]` lookup): a value
     like `HeroPump & ArmamentsX` (Veteran's Armaments) resolves a nil body and
     grants nothing. **23 corpus files** carry `AddSVar$ … &`.
     Note the value is an SVar NAME list whose bodies are
     `SVar:<Name>:<Value>`; split on the NAMES.
   - `AddStaticAbility$` (`rules/layers.go`, whole-value `fc.SVars[name]`): e.g.
     `t/tsagan_raider_warlord.txt` `AddStaticAbility$ SelfDT & WideFS`.
     **3 corpus files**.
   These are the identical grammar and the identical silent-zero-grant failure;
   this ticket deliberately did not widen its diff to fix them.
2. **`AddReplacementEffect$` is registered nowhere.** `grep -rn
   'AddReplacementEffect' rules/*.go` shows only a comment; the only code
   mention is the "unread" list in `effects/staticeffect.go`. **2 corpus files**
   carry the ` & ` form; a full census of all carriers is worth taking when the
   param is implemented. Separate ticket.
3. **CR-lane test worth adding** so this class stops being invisible to
   `.ds4/ledger.json`: CR 611.3 (continuous effects from a static ability) +
   CR 603.2 (a triggered ability granted by a static). Named only, not written
   (the brief did not ask for a CR-lane test).

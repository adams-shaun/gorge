# Report — task agent-20260919T062939Z-4b5f8950

**Ticket:** `RepeatOptional$` on `DB$ Repeat` — the may-repeat election is never posed
**Outcome:** the brief's premise is **FALSE**. Both halves of this ticket were
already implemented, merged to `main`, and covered by real-corpus tests before
this seat was dispatched. My branch HEAD is byte-identical to `main`; there is
no code to add. Nothing was committed because there was nothing to change.

## The measurement that settles it

```
$ git rev-parse HEAD main
582a564cf873993661b388d66e5f0390ff353e30
582a564cf873993661b388d66e5f0390ff353e30
$ git diff --stat HEAD main
(no output)
```

Both halves are in `git log --all`:

```
$ git log --oneline --all --grep='RepeatOptional' | cat
5fcdf7d0 merge(agent-20260922T194522Z-d7f24b09): `RepeatEach`'s `RepeatOptionalForEachPlayer$` is unread: the Tempt cycle
a98d1819 feat(effects): honour RepeatOptionalForEachPlayer$ in RepeatEach
46928423 merge(cli-20260922T150843Z-7fb23a6f): Repeat: read RepeatOptional$ as a repeat-again ask (Ad Nauseam)
8e77c2e8 test(botpolicy): pin the legal RepeatOptional$ election answer
7346c3ff fix(effects): pose the repeat election after a suspended body
```

The ledger confirms these are the **same two findings** as this ticket:

```
$ python3 (ledger.json slice)
{ "id": "issue-cli-20260922T150843Z-7fb23a6f",
  "title": "Repeat: read RepeatOptional$ as a repeat-again ask (Ad Nauseam)",
  "status": "closed", "disposition": "merged — 60005015" }
{ "id": "issue-agent-20260922T194522Z-d7f24b09",
  "title": "`RepeatEach`'s `RepeatOptionalForEachPlayer$` is unread: the Tempt cycle never a…",
  "status": "closed", "disposition": "merged — ca65ab38" }
{ "id": "issue-agent-20260919T062939Z-4b5f8950",   <-- THIS ticket
  "title": "RepeatOptional$ on DB$ Repeat — the may-repeat election is never posed",
  "status": "open", "disposition": "briefed — seat active", "priority": 4 }
```

This ticket is the ORIGINAL filing of what later became `7fb23a6f` (`DB$ Repeat`
half) and `d7f24b09` (`RepeatEach`/`RepeatOptionalForEachPlayer$` half). PCR: the
symptom in the brief — "`effRepeat` reads only `MaxRepeat`/`RepeatNum`" — is no
longer true. `effects/misc.go:2882` reads `RepeatOptional`, `poseRepeatOptionalElection`
(`effects/misc.go:3008`) poses the `KChoose` "Repeat this process?" election, and
`effects/choose_control.go:1565` reads `RepeatOptionalForEachPlayer` on `RepeatEach`.

## What changed and why (per file)

**Nothing.** No source file was modified. Writing an implementation here would
re-implement merged code and collide with the already-landed commit history.

## The brief's "Done means" is already met

`"a rules test where a real corpus carrier poses the election (the ask's answer
bounds the loop's iteration count) and the deterministic no-host fallback
repeats the documented count."`

- Real corpus carrier, answer bounds iteration count:
  `rules/repeat_optional_test.go::TestAdNauseamOptionalRepeatElectionStopsOnNo`
  (answering "no" resolves after **exactly one** body iteration — asserts the
  precondition that the body took 1 card *before* the election, then that no
  second card is taken after the stop) and `TestAdNauseamOptionalRepeatYesIterates`
  (answering yes runs the body a second time — proves the loop is a real do/while
  and the "no" test is not passing because iteration is impossible).
  `rules/repeat_each_optional_test.go::TestRepeatEachOptionalForEachPlayerMixedAnswers`
  covers the `RepeatEach` spelling on the real carrier `Tempt with Vengeance`.
- Deterministic no-host fallback: `effects/repeat_each_optional_test.go::TestRepeatEachOptionalForEachPlayerNoAskDeclines`
  ("the R-9 contract: a [host with no decision channel] declines each subject,
  never parks a suspension and never runs a body"). For the plain `RepeatOptional`
  path the fallback is one body iteration then stop (`poseRepeatOptionalElection`
  returns false → `effRepeat` returns), which is the documented R-9 shape in
  `docs/superpowers/specs/2026-09-22-engine-contracts.md`.

## Gate commands and real output

```
$ go test -v -run 'TestAdNauseamOptionalRepeatElectionStopsOnNo|TestAdNauseamOptionalRepeatYesIterates|TestForbiddenRitualBodyAskResumesToRepeatElection|TestRepeatEachOptionalForEachPlayerMixedAnswers|TestRepeatEachOptionalForEachPlayerDeclinesEverySubject|TestRepeatEachOptionalForEachPlayerAcceptsBoth|TestRepeatEachOptionalForEachPlayerSuspendedBody' ./rules/
=== RUN   TestRepeatEachOptionalForEachPlayerMixedAnswers
--- PASS: TestRepeatEachOptionalForEachPlayerMixedAnswers (0.60s)
=== RUN   TestRepeatEachOptionalForEachPlayerDeclinesEverySubject
--- PASS: TestRepeatEachOptionalForEachPlayerDeclinesEverySubject (0.00s)
=== RUN   TestRepeatEachOptionalForEachPlayerAcceptsBoth
--- PASS: TestRepeatEachOptionalForEachPlayerAcceptsBoth (0.00s)
=== RUN   TestRepeatEachOptionalForEachPlayerSuspendedBody
--- PASS: TestRepeatEachOptionalForEachPlayerSuspendedBody (0.00s)
=== RUN   TestAdNauseamOptionalRepeatElectionStopsOnNo
--- PASS: TestAdNauseamOptionalRepeatElectionStopsOnNo (0.00s)
=== RUN   TestAdNauseamOptionalRepeatYesIterates
--- PASS: TestAdNauseamOptionalRepeatYesIterates (0.00s)
=== RUN   TestForbiddenRitualBodyAskResumesToRepeatElection
--- PASS: TestForbiddenRitualBodyAskResumesToRepeatElection (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.684s
```

The 0.60 s first case (and the 0.68 s package total) is the corpus actually
loading — a `.cards`-less vacuous run would report ~0.00 s. `.cards` was already
present as a symlink to `/home/sadams/projects/gorge/.cards` (found present, not
created).

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.859s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.348s
```

No head/ratchet movement (nothing changed). `Ad Nauseam` is in the repo deck
`internal/testutil/decks/the-epic-storm.json`, so this behaviour is already in
the acceptance set. `Dance with Calamity` (the brief's named non-deck carrier)
is not in any repo deck, so no ratchet entry turns on it.

## Brief premise re-measured

- `RepeatOptional` corpus prevalence: brief says 14.
  `$ /usr/bin/grep -rl 'RepeatOptional' .cards/cardsfolder | wc -l` → **14** —
  held.
- The "symptom" (`effRepeat` reads only `MaxRepeat`/`RepeatNum`) is **false**
  at this HEAD: `effects/misc.go:2882` reads `RepeatOptional`, and the whole
  do/while + resume machinery is present.
- The `Dance with Calamity` carrier's `RepeatOptional$` election is now posed;
  its remaining gap is the separately-ticketed `api:GenericChoice` driver
  (`agent-20260918T202223Z-eb7aab2a`), exactly as the brief itself notes.

## Fails without the fix

Not applicable — no fix was made, so there is no hunk to revert. The existing
tests are already proven non-vacuous by their own precondition assertions (e.g.
`TestAdNauseamOptionalRepeatElectionStopsOnNo` asserts one library→hand move
*before* the election and one total after the stop; `TestRepeatEach…MixedAnswers`
asserts the declining opponent created 0 Elementals while the accepting one
created ≥1).

## Deviations from the brief

The brief asked for an implementation; none was owed. I did not write a
duplicate implementation, because that would re-land merged code and create a
merge conflict against `7fb23a6f`/`d7f24b09`. This is reported rather than
silently done (system-t1.md: "Reporting a brief's premise as false is a valued
outcome, not a failure to do the work").

## Issues

- **Duplicate ledger entry (bookkeeping, not code).** This ticket
  `agent-20260919T062939Z-4b5f8950` is the original filing of the two already-MERGED
  entries `cli-20260922T150843Z-7fb23a6f` and `agent-20260922T194522Z-d7f24b09`.
  It should be closed as superseded/duplicate. No code defect remains. (I cannot
  edit `.ds4/ledger.json` — it is derived — so this needs the controller.)
- **Minor test-coverage nuance (not a defect, not in scope).** The plain
  `DB$ Repeat` + `RepeatOptional$` **no-host** fallback (one body iteration then
  stop) has no dedicated rules-level R-9 test; only the `RepeatEach` spelling has
  `TestRepeatEachOptionalForEachPlayerNoAskDeclines`. The behaviour is correct
  (`effRepeat` returns when `h.Ask` reports no host) and the R-9 contract in
  `docs/superpowers/specs/2026-09-22-engine-contracts.md` does not name
  `RepeatOptional` explicitly. If the operator wants it pinned, a one-line
  addition to `effects/repeat_optional_test.go` running the fake host with
  `h.askResult = false` and asserting exactly one body run would do it. I did not
  write it: the brief's Done-means points at the corpus-carrier election, which
  is covered.
- **No new CR-lane test needed.** The feature is implemented, not approximated;
  there is no ledger-invisible defect here.

# Report — DestroyAll.Zone


# Task replcensus1 — api:ReplaceDamage census token

Ticket: `agent-20260919T055356Z-504b1359`
Branch: `wt/agent-20260919T055356Z-504b1359`
Commit: `b5c35e4d`

## What changed and why

### `rules/replacement.go` (the one production change)

Added `"api:ReplaceDamage"` to the `effects.RegisterNonAPI(...)` list inside
the package `init()`, with a comment naming the inline handler:

```go
"repl:AddCounter", "api:ReplaceCounter",
// api:ReplaceDamage is handled inline by applyReplaceDamageBody (this
// file) via the ReplaceDamage intercept in applyReplacements, never
// through effects.Resolve/runReplaceWith -- this registration is the
// census token only; a stub effects.Register handler would be dead code.
"api:ReplaceDamage")
```

Root cause as briefed: `rules/replacement.go`'s `applyReplacements`
intercepts a `ReplaceWith$` body whose API is `ReplaceDamage` and applies it
inline through `applyReplaceDamageBody`, so it never reaches
`effects.Register`; `effects.Supported()` therefore had no
`api:ReplaceDamage` and the census false-reported the 38 carrier cards as
unsupported. This is a census-token-only registration — no behaviour code
(`applyReplaceDamageBody`, the intercept) was touched, and no stub
`effects.Register` handler was added.

**Premap spot-check:** the brief's `Workspace facts` put the list at
`rules/replacement.go:4583` with `func init()` at 4545. At this base (main
`6ec869e5`) the list is actually at **line 6164** (`func init()` at 6126) —
the line numbers had drifted but the anchor (the `RegisterNonAPI` list
containing `"repl:AddCounter", "api:ReplaceCounter"`) was found and is
unique. Everything else in the premap held: `effects/registry.go` needed no
edit, and the four behaviour pins are untouched.

### `rules/replacedamage_registration_test.go` (new test file)

Per the "new tests go in a new file" rule, the pin lives in its own file
rather than appended to `coverage_test.go`:

- `TestReplaceDamagePrimitiveIsRegistered` — pins
  `effects.Supported()["api:ReplaceDamage"]`.
- `TestReplaceDamageCarrierHasNoGap` — loads the real corpus
  (`sharedCorpus`), finds Heart-Shaped Herb with the precondition
  `herb.Primitives()` contains `api:ReplaceDamage` (fails loudly if the card
  shape changes), then asserts `reg.Unsupported(herb, effects.Supported())`
  no longer contains `api:ReplaceDamage`.

## Gates run (real output)

### Targeted gate (Done-means command)

```
$ go test -v -run 'TestReplaceDamage|TestDamageReplacementSupportedBodyFamilies|TestBattletideAlchemist|TestThunderstaff|TestSpiderPunk' ./rules/
=== RUN   TestReplaceDamagePrimitiveIsRegistered
--- PASS: TestReplaceDamagePrimitiveIsRegistered (0.00s)
=== RUN   TestReplaceDamageCarrierHasNoGap
--- PASS: TestReplaceDamageCarrierHasNoGap (0.61s)
=== RUN   TestBattletideAlchemistAsksItsControllerAndPreventsClerics
--- PASS: TestBattletideAlchemistAsksItsControllerAndPreventsClerics (0.00s)
=== RUN   TestThunderstaffPreventsExactlyItsAmount
--- PASS: TestThunderstaffPreventsExactlyItsAmount (0.00s)
=== RUN   TestSpiderPunkStopsProtectionPrevention
=== RUN   TestSpiderPunkStopsReplaceDamagePreventionBodies
--- PASS: TestSpiderPunkStopsReplaceDamagePreventionBodies (0.00s)
=== RUN   TestDamageReplacementSupportedBodyFamilies
--- PASS: TestDamageReplacementSupportedBodyFamilies (0.00s)
ok  	github.com/adams-shaun/gorge/rules	0.661s
```

The corpus test took 0.61s and ran — not skipped (`.cards` was present as a
symlink; see "Workspace facts found"). All four pre-existing behaviour pins
pass.

### `## Fails without the fix`

Copied `rules/replacement.go` to `.ds4/scratch/replacement.go.bak`, removed
the registration line, ran only the new tests:

```
$ go test -run 'TestReplaceDamage' ./rules/
--- FAIL: TestReplaceDamagePrimitiveIsRegistered (0.00s)
    replacedamage_registration_test.go:21: effects.Supported() is missing "api:ReplaceDamage"
--- FAIL: TestReplaceDamageCarrierHasNoGap (0.65s)
    replacedamage_registration_test.go:38: Heart-Shaped Herb still reports api:ReplaceDamage unsupported: [api:ReplaceDamage]
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.668s
FAIL
```

Both new tests fail with the registration reverted. The file was restored and
verified byte-identical:

```
$ cmp .ds4/scratch/replacement.go.bak rules/replacement.go && echo "RESTORED BYTE-IDENTICAL"
RESTORED BYTE-IDENTICAL
```

### `go test ./internal/archtest/`

```
ok  	github.com/adams-shaun/gorge/internal/archtest	4.935s
```

No allowlist edits.

### `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`

```
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.305s
```

Byte-identical, as expected: a pure registration emits no event and no repo
deck carries a carrier.

### `gofmt -l` on touched files

```
$ gofmt -l rules/replacement.go rules/replacedamage_registration_test.go
(no output)
```

## Head / ratchet movement

None. `rules/acceptance_test.go` `knownUnsupported` and `rules/heads_test.go`
are untouched; no repo deck carries any of the 38 carriers (the corpus
probe and the empty `git diff` for those files confirm it). TestHeads was not
run (daemon gate), but a registration emits no event so no chain head can
move; `cmd/botbench`'s split stayed byte-identical, which is the same signal.

## Workspace facts found

- `.cards` was **present** (symlink resolved) at task start — the
  `TestReplaceDamageCarrierHasNoGap` run took 0.61s, proving the corpus
  loaded rather than skipped.
- The brief's PREMAP line numbers for `rules/replacement.go` had drifted from
  4583/4545 (main `18644593`) to 6164/6126 (base `6ec869e5`); the list was
  located by anchor and is the only such list in the file.
- Pre-existing worktree: `pwd` is the assigned worktree; `git status` was
  clean at start; no rebase was needed (branch was already at `6ec869e5`).

## Deviations from the brief

1. **Test lives in a new file, not `rules/coverage_test.go`.** The brief's
   Done-means says "New test in `rules/coverage_test.go`", but the dispatch's
   "New tests go in a new file (2026-09-22)" rule and the gorge context both
   require a new `_test.go` file to avoid merge conflicts with sibling
   tickets. The tests follow the `TestAddCounterReplacementPrimitivesAreRegistered`
   style exactly and use the same `sharedCorpus` helper. This is the only
   intentional deviation.

## Issues

No new defects found. The five carriers with other real gaps (Divine
Deflection, Errant Minion, Power Leak — `api:StoreSVar`; Nothing Can Stop Me
Now — `api:Abandon`; Urza Academy Headmaster —
`api:ControlPlayer`/`api:DamageResolve`/`api:SetLife`) were scoped out
per the brief and left untouched; the `api:StoreSVar` and `api:Abandon` gaps
are the known body-family remainders already tracked by other work, not new
findings.

```
STATUS=DONE
COMMITS=b5c35e4d
TESTS=go test -run 'TestReplaceDamage|TestDamageReplacementSupportedBodyFamilies|TestBattletideAlchemist|TestThunderstaff|TestSpiderPunk' ./rules/ → ok 0.66s (6 tests, incl. corpus carrier); new tests fail with the registration reverted; archtest + botbench byte-identical ok
```

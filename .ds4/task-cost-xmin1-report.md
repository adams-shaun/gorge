# Task cost-xmin1 — model Forge's `XMin<N>` announced-X lower bound

Branch worktree `agent-20260919T181318Z-3aa8759e`. Commit `b4f86aa0`
(`fix(rules): model Forge's XMin<N> announced-X lower bound`). No
attribution trailer, no `Ref:` trailer (gorge).

## What changed and why (per file)

`XMin<N>` is Forge's announced-X **lower bound** ("X can't be 0"), not a
payment. Before this fix only `suspendCost` knew the token (it strips it
before calling `ParseCost`); every other carrier fell through `ParseCost`'s
unrecognised-symbol fallback, which charged one phantom generic pip *and*
recorded a `Cost.Unknown` census label. The X announcement then offered
`X = 0` for cards whose text forbids it.

- **`rules/mana.go`**
  - New `var xMinCost = regexp.MustCompile(\`^XMin(\d+)$\`)` beside the other
    cost regexes (near `damageYouCost`/`rollDiceCost`).
  - `type Cost` gains `XMin int32` (scalar; verified `Cost` has no `==`
    comparison, no `map[Cost]` key, and is not a field of any
    event/protocol/view struct).
  - `ParseCost`: a new arm **before** the numeric fallback matches
    `XMin<N>`, clamps `N` the way every other arm does, sets `Cost.XMin`,
    adds no generic and calls no `reportUnknown`, then `continue`s. A
    malformed/overflowing instance keeps the ordinary reported one-generic
    fallback.
  - `Cost.WithX`: clears `XMin` after folding the chosen X, so a later
    `WithX` cannot re-charge the floor as generic. (Decision stated: the
    bound is consumed by the announcement; `xAsk` reads it off the
    **unfolded** cost.)
  - `Cost.Plus`: `XMin` becomes the **max** of the two bounds — the shared
    announced X takes the highest floor (kicked Thieving Skydiver: printed
    cost has none, Kicker part has `XMin1`).
- **`rules/statics.go`** — `costMods.feasibleAny`'s shared `composed` leaf
  prices a cost carrying `XMin > 0` at `WithX(XMin)` on a **local copy**
  before the modifier composition. This is the one shared choke point every
  mana-feasibility check goes through. It is deliberately *inside*
  `composed` and not at `offerCastableUsing`: folding in the engine entry
  would rebuild the payment descriptor from a folded cost and hide every
  `CostContainsX` batch from the target-repricing gates — the exact trap
  the `statics.go:1618` comment names. Folding on the local copy leaves the
  descriptor (built by `paymentFor` from the raw cost in
  `manaFeasiblePriced`) reporting the announced-X marker intact.
- **`rules/cast.go`** — `xAsk`'s option-list floor is now
  `max(pc.cost.XMin, suspendMinX-when-suspend)` instead of the suspend-only
  bound. The Suspend special case in `suspendCost` is untouched; its
  existing test still passes.
- **`rules/xmin_cost_test.go`** (new) — parse leaf, corpus-carrier leaf
  (Thieving Skydiver), and inline-fixture offer-gate leaf.
- **`rules/paramcensus_test.go`** — the `TestParseCostReportsUnmodelledCostTokens`
  table gains `{"XMin1 X", nil}` and `{"XMin4", nil}` as the brief asks.

Structural approach (for the "fix the class, not the instance" requirement):
the bound lives on `Cost` and is enforced at the two shared choke points
(`feasibleAny`'s composed leaf and `xAsk`'s floor), not per carrier. Any
future `XMin<N>` carrier — Kicker, Flashback, a raw `A:*/Cost$`,
`UnlessCost$` — is covered by the same parse + `Plus` + gate path with no
per-keyword arm.

## Brief premises re-measured

- Corpus: `29` files carry `XMin[0-9]+`: `27 XMin1`, `2 XMin4`. Held.
- No `internal/testutil/decks/*.json` names any XMin carrier (checked by
  grep). Held — no repo deck imports Grand Larceny, as the brief states, so
  the param census ratchet cannot move and `knownUnsupportedParams` was not
  touched.
- No `AGENTS.md` Known-approximations row mentions `XMin` (`grep -in xmin
  AGENTS.md` → none). Held — nothing to delete, no
  `knownApproximationRows` change.
- Workspace fact: the brief's HEAD was `141de55c`; this worktree is at
  `6c14e326` (the fuzz-cov3 merge). The named anchors were all present with
  minor line drift; I spot-checked each before editing.

## Exact gate commands and real output

Targeted run (brief's command, real test names), plus my new tests:

```
$ go test -run 'TestParseCostReportsUnmodelledCostTokens|TestKicker|TestSuspendX|TestThievingSkydiver' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.436s
```

```
$ go test -run 'TestParseCostModelsXMinLowerBound|TestThievingSkydiverKickedXMinOffersOnlyNonzeroX|TestXMinKickerOfferGateWithholdsUnpayableMinimum|TestParseCostReportsUnmodelledCostTokens|TestSuspendXBenalishCommanderAnnouncesTimeAndCost|TestParamCensusCatchesFaceOwnedCosts|TestParamCensusPinsTheImportReviewExamples' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.624s
```

Cost-family regression subset (feasibleAny/WithX/Plus are shared):

```
$ go test -run 'TestConvoke|TestToxicDeluge|TestDelve|TestBestow|TestEmerge|TestBuyback|TestMultikicker|TestReplicate|TestSurge|TestXMin' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.486s
```

Golden gates outside `rules/`:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.721s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.626s
```

Format/type checks:

```
$ gofmt -l rules/mana.go rules/statics.go rules/cast.go rules/xmin_cost_test.go rules/paramcensus_test.go
(no output)

$ go run ./cmd/gentypes -check
(no output, exit 0)
```

## Fails without the fix

I reverted the behavioural hunks in two stages (keeping the `Cost.XMin`
field so the tree compiles), ran the three new tests, and restored the
files byte-identically (`cmp` clean against saved copies). No `git stash` or
`git checkout <path>` was used.

**Stage 1 — the parse arm removed (field/regex/Plus/WithX kept).** The
reported phantom pip reappears:

```
--- FAIL: TestParseCostModelsXMinLowerBound (0.00s)
    xmin_cost_test.go:52: ParseCost("XMin1 X B").Generic = 1, want 0
    xmin_cost_test.go:58: ParseCost("XMin1 X B").XMin = 0, want 1
    xmin_cost_test.go:64: ParseCost("XMin1 X B").Unknown = [XMin1], want empty
    xmin_cost_test.go:52: ParseCost("XMin1 X").Generic = 1, want 0
    ... (XMin1, XMin4 identical)
    xmin_cost_test.go:78: Plus XMin composition = {... Generic:2 ... XMin:0 ...}, want XMin1 X1 Generic1
--- FAIL: TestThievingSkydiverKickedXMinOffersOnlyNonzeroX (0.41s)
    xmin_cost_test.go:106: Thieving Skydiver Kicker parse = {... Generic:1 ... XMin:0 ... Unknown:[XMin1]}, want XMin1 X1 Generic0
--- FAIL: TestXMinKickerOfferGateWithholdsUnpayableMinimum (0.00s)
    xmin_cost_test.go:151: fixture Kicker parse = {... Generic:1 ... XMin:0 ... Unknown:[XMin1]}, want XMin1
FAIL
```

**Stage 2 — only the `feasibleAny` fold and the `xAsk` floor reverted
(the parse fix present, so the preconditions pass).** This isolates the two
engine hunks:

```
--- FAIL: TestThievingSkydiverKickedXMinOffersOnlyNonzeroX (0.43s)
    xmin_cost_test.go:117: kicked XMin1 ask offered [{... Label:X = 0 ... Amount:0} {... Label:X = 1 ... Amount:1}], want only X = 1
--- FAIL: TestXMinKickerOfferGateWithholdsUnpayableMinimum (0.00s)
    xmin_cost_test.go:161: kicked offered on {1}{U} (modes [ kicked]): its minimum X=1 needs {2}{U}
FAIL
```

So the parse test fails without the parse arm; the Thieving Skydiver test
fails at the X ask (`X = 0` offered) without the `xAsk` floor and at the
offer gate without the fold; the offer-gate test fails without the fold.
Each new test is proven to be able to fail, and every new test asserts its
precondition (Kicker parses to `XMin==1`; the fixture is in hand; the plain
cast stays offered alongside the withheld kicked option).

## Fails-without-fix hygiene

- Backups `.ds4/scratch/{mana,statics,cast}.go.fixed`; restored and `cmp`ed
  byte-identically before the green run was retaken.
- No file was left in the reverted state in the commit.

## Report contract / other Done-means items

- No Known-approximations row closed (none names `XMin`), so
  `internal/testutil/agentsdoc_test.go`'s `knownApproximationRows` is
  unchanged. No row added or grown.
- `.cards` existed as a symlink
  (`-> /home/sadams/projects/gorge/.cards`) — corpus tests ran, they did not
  skip. (The `rules` runs are ~0.4–0.6 s for a targeted subset; the corpus
  itself did load, proven by the Thieving Skydiver corpus lookup passing.)
- No repo deck carries an XMin carrier, so `cmd/botbench`'s pinned split was
  not expected to move; `TestConstructedDefaultIsByteIdentical` passed, so
  nothing was re-pinned.
- `go test ./internal/archtest/` passed with no allowlist edit.

## Issues

1. **Direct-proposal X ask can pose an empty option list when `XMin > 0`
   and the pool cannot pay the floor.** `xAsk` builds `for x := min; x <=
   bound` and its empty-fallback `for x := min; x <= maxOld`; if a caller
   invokes `beginCast` directly on a kicked `XMin1` cost with a pool short of
   `{XMin}` (the CR 733.1 audit path does exactly this), both ranges are
   empty and the posed `KChoose` has no options (`Min:1 Max:1`). The offer
   gate now withholds that cast, so it is unreachable through
   `legalActions`; the direct-proposal shape mirrors the pre-existing
   Suspend X-time behaviour and is not a new regression. Worth a follow-up
   (`rules/cast.go` `xAsk`) that aborts the cast loudly instead of posing an
   unanswered ask. Not fixed here (outside the brief; changing the abort
   direction would move CR 733 behaviour).
2. **`RemoveAnyCounter<X1+/…>` is a distinct unmodelled token** (Ooze Flux's
   `Cost$ XMin1 1 G RemoveAnyCounter<X1+/…>`; the X only reaches an
   unmodelled part). `removeAnyCounterCost` accepts only `X` or digits, so
   `X1+` does not match and is itself the reported fallback. Explicitly out
   of scope; the brief says ticket
   `agent-20260919T195359Z-695edfb2` owns `RemoveAnyCounter`.
3. **`K:Craft:<n> … XMin1 …` carriers** (6 files: altar_of_the_wretched,
   paleontologists_pick_axe, sunbird_standard, saheelis_lattice,
   ore_rich_stalactite, the_enigma_jewel) get only the phantom-pip half of
   this fix removed. Craft has no cast-mode path found in `rules/`, so its
   XMin is parsed onto `Cost.XMin` but never announced. Out of scope per the
   brief.
4. **`cost:XMin1` will not appear (or disappear) in the census for
   `K:Suspend`/`K:Craft`** because `faceUnknownCostLabels` does not scan
   those keywords — `rules/paramcensus_test.go`. They never did; after the
   fix the token is modelled rather than reported for the scanned carriers.
   Stated so nobody expects a census row.
5. **Grand Larceny (otc, Gonti, Canny Acquisitor) is still not imported**
   into `internal/testutil/decks/`; the deck-census ticket committed only
   `deck.list`. A separate effort; this ticket adds no repo deck.
6. **`XMin<N>` on a cost with no `{X}` symbol** (only possible when the
   `XMin<N>` token is the whole cost, or the X reaches only an unmodelled
   part — Ooze Flux) sets `Cost.XMin` but announces nothing, so the floor
   is inert. Harmless and fail-safe; noted so a future carrier that relies
   on the bound without a printed `{X}` is not silently mis-modelled.

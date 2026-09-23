# Vanishing implementation report

## Changes

- `cards/kw_vanishing.go`: registered a `Vanishing` expander for the scoped `Vanishing:<N>` form. It adds a battlefield-entry TIME-counter replacement, a controller-upkeep Phase trigger gated on at least one TIME counter, and a separate battlefield `CounterRemoved` trigger gated on the post-removal count being zero. Removal and sacrifice are ordinary triggered effects; bare `K:Vanishing` is deliberately not treated as a count-bearing form.
- `cards/kw_registry_test.go`: added `Vanishing` to the expander registry ratchet.
- `cards/kw_vanishing_test.go`: verifies expansion into entry replacement plus the upkeep and last-counter triggers.
- `rules/vanishing_test.go`: corpus-backed Deep Forest Hermit tests assert battlefield placement and its three entry counters; verify upkeep removal resolves through the stack and the last-counter sacrifice is a separate trigger; check another player's upkeep and zero-counter upkeep do not tick or queue the Vanishing trigger.

`.cards` was present (not skipped). Measured 21 corpus files containing `K:Vanishing`; of those script lines, 19 use `K:Vanishing:<N>` and two are bare `K:Vanishing` (Out of Time and Tidewalker). Repo-deck ratchets and heads were not edited. No Known approximations row was closed or changed.

## Fails without the fix

Copied `cards/kw_vanishing.go` to `.ds4/scratch/kw_vanishing.go.fixed`, removed the production expander, ran the required targeted command, then restored and verified the file byte-identically:

```text
exit=1
--- FAIL: TestEveryExpandedKeywordHasAnExpander (0.00s)
    kw_registry_test.go:84: keyword head "Vanishing" has no registered expander: it silently stops expanding
--- FAIL: TestVanishingExpansion (0.00s)
    kw_vanishing_test.go:9: Vanishing entry replacement = [], want ETB placement of 3 TIME counters
FAIL
FAIL	github.com/adams-shaun/gorge/cards	0.002s
--- FAIL: TestVanishingDeepForestHermitUpkeepAndLastCounter (0.58s)
    vanishing_test.go:38: precondition: Deep Forest Hermit entered with 0 TIME counters, want 3
--- FAIL: TestVanishingOnlyTriggersOnControllersUpkeepAndNotAtZero (0.00s)
    vanishing_test.go:75: precondition: Deep Forest Hermit entered with 0 TIME counters, want 3
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.607s
restored byte-identically
```

## Verification

Targeted command:

```text
go test -run 'TestEveryExpandedKeywordHasAnExpander|TestVanishing' ./cards/ ./rules/
exit=0
ok  github.com/adams-shaun/gorge/cards  0.002s
ok  github.com/adams-shaun/gorge/rules  0.619s
```

Architecture gate:

```text
go test ./internal/archtest/
exit=0
ok  github.com/adams-shaun/gorge/internal/archtest  (cached)
```

Botbench golden:

```text
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
exit=0
ok  github.com/adams-shaun/gorge/cmd/botbench  1.231s
```

Formatting/type generation checks:

```text
gofmt -l cards/kw_vanishing.go cards/kw_vanishing_test.go cards/kw_registry_test.go rules/vanishing_test.go
(no output)
go run ./cmd/gentypes -check
(no output; exit 0)
```

`git diff --check` passed with no output. No botbench split movement.

## Issues

- Bare `K:Vanishing` appears on 2 corpus cards (Out of Time and Tidewalker). It has no `<N>` count and is outside this ticket's explicitly scoped `Vanishing:<N>` script shape; the expander intentionally returns without inventing behavior for it. A follow-up must define the intended bare-keyword semantics before implementing it.

## Commit

`3f4072f3 feat(cards): implement Vanishing time counters`

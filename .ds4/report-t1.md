# Report — TriggerController$ on ChangesZone

## Changes

- `rules/trigger_match.go`: when a `ChangesZone`/`ChangesZoneAll` trigger explicitly names `TriggerController$ TriggeredCardController`, the queued ability's controller (and matching `Ctx.Controller`) now comes from the moved permanent's LKI on a battlefield departure. This keeps APNAP grouping, stack control and resolution under that controller, without changing default trigger controller behavior or interpreting other selector values.
- `rules/trigger_controller_test.go`: added a focused integration test using an inline watcher script. A seat-1 watcher sees a creature stolen by seat 0 die; it asserts the creature is on the battlefield before departure, the two controllers differ, its live controller has reset to its owner after moving, the pending/stack trigger belongs to seat 0 rather than the watcher, and resolution affects only seat 0.

The parser/read census needed no special table change. The ordinary `t.Params["TriggerController"]` access is included by the static trigger-parameter census; `TestEveryRepoDeckParamsAreRead` passes with no new unsupported entry. GNU grep measured 42 corpus files with `TriggerController$`.

A direct Junji corpus test was not added: Junji's trigger is on the same card that dies, so the existing default leaves-the-battlefield LKI controller path already assigns it to the departing card's last controller. Such a test would pass with this fix reverted and violate the required fail-without-fix proof. The new inline watcher test isolates and proves the selector's distinct behavior. Junji's compiled script is present in `.cards`.

## Fails without the fix

Saved the fixed production file, removed only the new controller-selection hunk, ran the focused test, and restored the file byte-identically (`cmp` exit 0):

```text
go test -run '^TestChangesZoneTriggeredCardControllerUsesDepartingCardLKI$' ./rules/
--- FAIL: TestChangesZoneTriggeredCardControllerUsesDepartingCardLKI (0.00s)
    trigger_controller_test.go:32: trigger controller = 1, want departing card's last controller seat 0 (witness controller is seat 1)
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.003s
```

## Gates and output

Corpus present: `.cards` is a symlink to `/home/sadams/projects/gorge/.cards`.

```text
/usr/bin/grep -rlE 'TriggerController\\$' .cards/cardsfolder | wc -l
42

gofmt -l rules/trigger_match.go rules/trigger_controller_test.go
[no output]
go run ./cmd/gentypes -check
[no output]

go test -run 'TestChangesZoneTriggeredCardControllerUsesDepartingCardLKI|TestEveryRepoDeckParamsAreRead' ./rules/
ok github.com/adams-shaun/gorge/rules 0.750s

go test ./internal/archtest/
ok github.com/adams-shaun/gorge/internal/archtest 3.489s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok github.com/adams-shaun/gorge/cmd/botbench 1.334s

git diff --check
[no output]
```

Botbench split remains byte-identical; no golden update was needed. No Known-approximations row is closed by this change.

## Issues

No unfixed behavior identified within the requested `TriggeredCardController` selector for battlefield departures. Other `TriggerController$` spellings and ChangesZone events that are not battlefield departures remain outside this implementation's scope; the selector is intentionally limited to the substantiated form and event/LKI boundary.
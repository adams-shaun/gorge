# trig:SearchedLibrary implementation report

## Changes

- `events/event.go`, `events/apply.go`: appended `SearchedLibrary` after the existing final event kind and made it an Apply no-op marker. Existing event ordinals remain unchanged.
- `effects/zone.go`: `applyLibrarySearch` emits one marker per completed searched library, including searches with no eligible card. It is distinct from card movement, so ordinary library moves cannot masquerade as searches.
- `rules/trigmatch_cards.go`, `rules/trigger_eligibility.go`, `rules/trigger_match.go`: implemented and registered `Mode$ SearchedLibrary`, matching the marker's searched player and source card against `ValidPlayer$` and `ValidCard$`. Added the trigger's non-API support registration.
- `rules/trigmatch_registry_test.go`, `rules/trigger_eligibility_test.go`: joined the post-split matcher ratchet and event eligibility matrix.
- `rules/searched_library_trigger_test.go`: added an end-to-end test using Evolving Wilds to perform a real search and River Song as the opponent's trigger source; it asserts both are correctly situated, exactly one marker and trigger push occur, and the game replays.

Structural choice: the trigger keys off a dedicated logged marker emitted by the shared library-search completion function, not search-specific card MoveZone records. This covers the existing and future library-search callers uniformly and avoids false positives from ordinary card movement.

The corpus prevalence premise held: `grep -Rln 'Mode\$ SearchedLibrary' .cards/cardsfolder | wc -l` returned `4`.

## Verification

Corpus was present in this worktree (`.cards` existed; corpus-dependent test did not skip).

- `go test -run 'TestRiverSongOpponentSearchFiresOnce$|TestTriggerEligibilityEventMatrix$|TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched$' ./rules/`

  ```text
  ok   github.com/adams-shaun/gorge/rules  0.632s
  ```

- `make report`

  ```text
  corpus: 95f04e8a04c8925fa97cb226fc3341cabcc90a53 @ 95f04e8a04c8925fa97cb226fc3341cabcc90a53 (GPL-3.0, 33669 files)
  cards: 33667  playable: 29673 (88.1%)
  tokens: 839
  ```

  The report no longer lists `trig:SearchedLibrary` among missing primitives. `forgec` rebuilt the IR because the ignored cache was older than the current IR version, then reported from the corpus.

- `go test ./internal/archtest/`

  ```text
  ok   github.com/adams-shaun/gorge/internal/archtest  3.339s
  ```

- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`

  ```text
  ok   github.com/adams-shaun/gorge/cmd/botbench  1.157s
  ```

- `gofmt -l <changed Go files>`: no output.
- `go run ./cmd/gentypes -check`: exit 0, no output.
- `git diff --check`: exit 0, no output.

## Fails without the fix

Saved `effects/zone.go`, removed only the SearchedLibrary marker emission, ran the regression, then restored the file and verified it byte-identically with `cmp`.

Command: `go test -run '^TestRiverSongOpponentSearchFiresOnce$' ./rules/`

```text
--- FAIL: TestRiverSongOpponentSearchFiresOnce (0.88s)
    searched_library_trigger_test.go:44: completed Evolving Wilds search emitted 0 SearchedLibrary markers, want 1
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.909s
FAIL
```

## Issues

None found outside the requested trigger implementation; no unaddressed deviations.

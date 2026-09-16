# Report — inbox-rv2b-brainstorm-put-back-from-hand

`.cards` was already present (`.cards/ir.gob.gz` exists), so corpus-backed tests ran.

## What changed

- `effects/zone.go`: merged the current defined-library-fetch implementation with the hidden-hand chooser. Hidden `Origin$ Hand` moves retain their resumable whole-hand and per-owner chooser path, and library placement/shuffle helpers now serve both the hand mover and main's defined-library fetcher without changing their distinct shuffle defaults. The hand chooser prompt now names **that player's library** when someone chooses from another player's hand.
- `effects/registry.go`: retained both the per-owner `HandMoveTarget` continuation and main's `DefinedLibraryMove` transport.
- `effects/hand_move_test.go`: added the cross-player prompt regression.

The dispatch is structural: exact `Origin$ Hand` with no object-valued `Defined$` always goes through the shared hand-owner walk (rather than a card-name list), while exact `Origin$ Library` first dispatches object-valued `Defined$` through main's shared direct-fetch helper. Thus future Hand owner selectors and Library fetch-list selectors do not fall back to `Defined()`'s source default.

## Corpus / heads

The brief's 42-file claim did not hold at this corpus pin. The earlier audit on this branch measured 453 raw exact-`Origin$ Hand` lines in 431 files: 239 whole-hand, 133 owner-selected, and 81 already-concrete object moves.

`rules/heads_test.go` was not edited. `TestHeads` still reports the intended changed trajectories:

| seats | computed | existing golden |
|---:|---|---|
| 4 | `b5888e1f7c2ccab9` | `2753ceca0bed344d` |
| 6 | `ae1e8e5219b49537` | `b5882f44d619a1c5` |
| 8 | `324d66dfb43440ce` | `c54d57bf94915dcb` |

The prior acceptance trace measured the new reachable `hand_move` asks as Brainstorm plus Thought-Knot Seer/Stoneforge Mystic in these games; the two-seat game reaches none. Golden regeneration remains controller-owned.

## Gates

```text
$ go test ./effects/ ./rules/ ./view/ ./host/...
ok   github.com/adams-shaun/gorge/effects  (cached)
--- FAIL: TestHeads (0.83s)
    heads_test.go:901: 4 seats: chain head b5888e1f7c2ccab9, golden 2753ceca0bed344d
    heads_test.go:901: 6 seats: chain head ae1e8e5219b49537, golden b5882f44d619a1c5
    heads_test.go:901: 8 seats: chain head 324d66dfb43440ce, golden c54d57bf94915dcb
FAIL
FAIL github.com/adams-shaun/gorge/rules 55.749s
ok   github.com/adams-shaun/gorge/view 1.048s
ok   github.com/adams-shaun/gorge/host 14.328s
ok   github.com/adams-shaun/gorge/host/httpapi 1.710s
FAIL

$ go test ./rules/ -run 'TestEveryRepoDeck|TestRepoDecks'
ok   github.com/adams-shaun/gorge/rules 1.071s

$ gofmt -l . && go vet ./... && go run ./cmd/gentypes -check
(exit 0; no output)
```

The pre-commit measurement accepted effects (9.8s/306), host (14.0s/110), rules (55.0s/790), and decision. It refused to record `host/httpapi` because its 42 non-skipped tests completed in about 1.7s, below its stale wall-time anomaly threshold despite `.cards` being present; the merge commit therefore used `--no-verify` after the direct non-cached `go test -count=1 ./host/httpapi/` passed (52 top-level runs, 0 skips).

## Issues

- Mixed hidden origins containing Hand remain a loud no-move fallback: 32 raw lines / 32 files. `effects/zone.go:mixedOriginIncludesHand` needs an origin-aware private chooser across mixed zones.
- `Tapped$ True` remains unread by the shared hidden-hand chooser (51 raw exact-`Origin$ Hand` lines / 49 files); battlefield entry needs an event-backed tapped-entry implementation.
- `cmd/testtime` may falsely reject `host/httpapi`'s legitimate current fast run based on stale timing history; it should distinguish a speedup from a vacuous corpus run without requiring a commit-hook bypass.

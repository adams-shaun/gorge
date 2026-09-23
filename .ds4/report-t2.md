# Report — player-count sacrifice attribution finding

## Changes

- `effects/count.go`: removed `SacrificedThisTurn` from the resolved `PlayerCountPropertyYou$` cases. With no replay-stable actor attribution, it now retains the fail-closed `(0, false)` verdict. The other three task properties (`CardsDiscardedThisTurn`, `LifeLostThisTurn`, `LandsPlayed`) remain resolved.
- `effects/registry.go`, `rules/stack.go`: removed the `Host.SacrificesThisTurn` contract and the owner-attributed event fold. A sacrifice event does not identify the acting player; an object's owner or controller cannot substitute for that actor.
- `effects/context_test.go`, `effects/playercount_property_you_test.go`: removed the fake sacrifice tally, retained the other per-turn ledger assertions, and added an unresolved-verdict assertion for `SacrificedThisTurn`.
- `rules/sacrifices_this_turn_test.go`: replaced the old owner-count/reset test with a regression that establishes a battlefield permanent owned by player 1 and controlled by player 0, emits a canonical sacrifice event, and asserts player 0's count remains unresolvable.

This directly addresses the finding: the previous count credited owner rather than the player taking the action. I chose the finding's explicitly permitted conservative path instead of changing the append-only/hash-chained event encoding. Exact actor attribution requires a separately designed replay-stable provenance mechanism.

`.cards` was present in this worktree. I first checked the clean worktree and rebased onto `main` as directed. The rebase conflicted only in the ignored agent report `.ds4/report-t1.md`; I preserved both sides' content and continued successfully.

## Gates run

```text
$ go test -run '^TestPlayerCountPropertyYouPerTurnLedgerCounts$' ./effects/
ok   github.com/adams-shaun/gorge/effects  0.002s

$ go test -run '^TestSacrificedThisTurnRemainsUnresolvedWithoutActorProvenance$' ./rules/
ok   github.com/adams-shaun/gorge/rules  0.002s

$ go test ./internal/archtest/ 2>&1 | tail -15
ok   github.com/adams-shaun/gorge/internal/archtest  5.524s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok   github.com/adams-shaun/gorge/cmd/botbench  1.836s

$ gofmt -l effects/count.go effects/registry.go effects/context_test.go effects/playercount_property_you_test.go rules/stack.go rules/sacrifices_this_turn_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output; exit 0)
$ git diff --check
(no output; exit 0)
```

No botbench split change was observed. `TestHeads` and the deck acceptance ratchet were not run; they are daemon gates, and the original task's Evendo acceptance is still blocked by its separate `Card.ExiledWithSource` matcher.

## Fails without the fix

For the effects regression, I backed up the changed implementation files, restored the pre-fix `3d081fa4` versions of `effects/count.go`, `effects/registry.go`, and `effects/context_test.go`, and ran the targeted test. I restored the fixed files and verified each with `cmp` (`restore_cmp=0`):

```text
$ go test -run '^TestPlayerCountPropertyYouPerTurnLedgerCounts$' ./effects/
--- FAIL: TestPlayerCountPropertyYouPerTurnLedgerCounts (0.00s)
    playercount_property_you_test.go:52: SacrificedThisTurn = (0, true), want unresolved (0, false) without actor provenance
FAIL
FAIL github.com/adams-shaun/gorge/effects 0.002s
FAIL
```

For the owner/controller mismatch regression, I similarly restored the pre-fix `3d081fa4` versions of `effects/count.go`, `effects/registry.go`, `effects/context_test.go`, and `rules/stack.go`, then restored and byte-compared all four fixed files (`restore_cmp=0`):

```text
$ go test -run '^TestSacrificedThisTurnRemainsUnresolvedWithoutActorProvenance$' ./rules/
--- FAIL: TestSacrificedThisTurnRemainsUnresolvedWithoutActorProvenance (0.00s)
    sacrifices_this_turn_test.go:30: player 0 SacrificedThisTurn = (0, true), want unresolved (0, false) without actor provenance
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.003s
FAIL
```

## Issues

- The original Evendo Brushrazer gate remains inert because `Affected$ Card.ExiledWithSource` is still unmatched; the prior task measured raw text in 100 `.cards/cardsfolder` scripts. Resolving this count alone would not enable the card.
- `PlayerCountPropertyYou$SacrificedThisTurn` is intentionally unresolved until exact replay-stable actor provenance exists. Other non-HasPropertyActive family members (including combat-history and Attractions properties) also remain unsupported; this round did not expand their semantics.
- No Known approximations row was changed; the original task did not own a row for this count.

# Report — inbox-engine-gap-replacement-turn-mana (integration round 5)

Worktree fixture: `.cards` was already present, so corpus-backed tests ran.

## What changed

- Merged current `main` at `897531c` and reconciled the replacement implementation with its new Madness, action-marker, LKI, control, mana-activation, and turn APIs.
- `rules/replacement.go`: retains the ticket's Untap, BeginPhase, Transform, ProduceMana, and replacement-mana state machines; composes Madness dispatch and discard-only Moved gates; carries replacement action markers; and uses current trigger/LKI signatures.
- `rules/turn.go`, `rules/engine.go`, `effects/registry.go`: retain replacement parking/provenance while preserving current control and mana-choice state. An existing priority decision is not mistaken for a newly parked Untap replacement during direct turn-driving tests.
- `rules/acceptance_test.go`: uses main's ratchet changes and removes the ticket-completed BeginPhase and Virtue labels. Necropotence is now fully supported on main, so no stale entry remains.

The structural integration keeps all ProduceMana provenance in the sole mana-ability resolution path and all replacement continuation handling in `replChoice`/`handleReplacement`; it is not card-name-specific.

## Corpus prevalence

```text
$ printf 'Untap='; /usr/bin/grep -rl '^R:Event\$ Untap' .cards/cardsfolder | wc -l; printf 'BeginPhase='; /usr/bin/grep -rl '^R:Event\$ BeginPhase' .cards/cardsfolder | wc -l; printf 'Transform='; /usr/bin/grep -rl '^R:Event\$ Transform' .cards/cardsfolder | wc -l; printf 'ProduceMana='; /usr/bin/grep -rl '^R:Event\$ ProduceMana' .cards/cardsfolder | wc -l; printf 'ReplaceMana='; /usr/bin/grep -rl 'DB\$ ReplaceMana' .cards/cardsfolder | wc -l
Untap=156
BeginPhase=21
Transform=4
ProduceMana=11
ReplaceMana=22
```

These match the prior measured report counts. Ticket real-card tests remain in `rules/replacement_turn_mana_test.go` for Basalt Monolith, Necropotence, Sephiroth, Fabled SOLDIER, Virtue of Strength, plus Damping Sphere and replacement-interaction regressions.

## Gates

```text
$ go test ./rules/
ok  github.com/adams-shaun/gorge/rules  62.696s

$ go test ./rules/ -run 'TestEveryRepoDeckIsFullySupported$' -v
=== RUN   TestEveryRepoDeckIsFullySupported
    acceptance_test.go:163: ratchet: 41 of 579 distinct cards across the repo decks are not fully supported
--- PASS: TestEveryRepoDeckIsFullySupported (0.46s)
PASS
ok  github.com/adams-shaun/gorge/rules  0.480s

$ go test ./view/
ok  github.com/adams-shaun/gorge/view  1.150s

$ go test ./rules/ -run 'TestHeads$' -v
=== RUN   TestHeads
--- PASS: TestHeads (0.92s)
PASS
ok  github.com/adams-shaun/gorge/rules  0.925s

$ make sim 2>&1 | grep -c 'replay OK'
20

$ gofmt -l . && go vet ./... && go run ./cmd/gentypes -check
```

The static command exited zero with no output. `TestHeads` passed unchanged; no golden was edited.

The merge commit hook also completed all changed package measurements; its rules result was `59.8s 782 tests 1 skipped budget 124s`.

## Issues

1. **Bombur, Gentle Dreamer's Enduring Story gate remains unsupported.** `.cards/cardsfolder/b/bombur_gentle_dreamer.txt` is the sole raw corpus file with both `R:Event$ Untap` and `EnduringStory$` (`/usr/bin/grep -rlE '^R:Event\$ Untap.*EnduringStory\$' .cards/cardsfolder | wc -l` = 1). `rules/replacement.go:replacementMatches` does not read `EnduringStory$`, so it prevents untapping regardless of that condition. Enduring-story state is outside this ticket; a future CR regression should cover its conditional “unless” behavior.

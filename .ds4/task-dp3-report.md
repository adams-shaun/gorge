# dp3c — NEEDS_CONTEXT: safe target-effect facts cross an ownership boundary

## Found state and merge

Found a clean tree at `fce341a0f941416f99258e36ddbaba4ecaf9cc5e`, with all seven implementation files preserved in that checkpoint. Read its complete diff. No previous `.ds4/task-dp3-report.md` existed. `.cards/ir.gob.gz` is reachable; the acceptance tests ran rather than skipped.

Merged `main` at `0751802152322823156c09cf81d77b9136e632cc` with `git merge --no-ff main`, without rebasing/amending the checkpoint. Merge commit: `688bc6d`. The only conflict was `botpolicy/cast.go`. Kept both dp2's mana production and dp1b's worth/tax helpers. Kept the checkpoint's effect-class bonus LOCAL to `castScore`, on top of unchanged `cardWorth`. Post-merge tests ran before any further implementation. No further implementation was attempted after identifying the ownership blocker below.

## Controller decision needed

The inherited face implementation is NOT safe to ship:

1. `faceScore` treats any player at <=3 life as a winning target without knowing whether the active effect deals damage, how much damage it deals, or whether it is prevented. Even missing life information reads zero and receives a lethal bonus.
2. It treats proximity to 21 commander damage as burn reach. Ordinary spell damage does not advance the commander COMBAT damage clock. Its `TestTargetFaceNearCommanderClock` currently asserts this incorrect behavior.
3. Its multiplayer face score depends on the target player's largest creature, so it can prefer the healthier player's face because that player owns a bigger creature.
4. The non-creature classifier is an oracle-phrase approximation, not answer-aware removal valuation. Broad `exile`/`destroy` matching can misclassify costs or unrelated abilities; the class bonus does not inspect what removal actually answers. `Card.Types` is projected but the classifier does not actually read it.

I inspected `decision/decision.go`, `rules/stack.go:173-270`, and both adapter fills. `askTarget` has the exact active `*cards.SA`, but only emits Source plus legal target identities. It exposes neither that active effect nor its damage amount/effect semantics. `Board.Cards` includes the deciding player's hand/graveyard/battlefield/command, NOT stack objects, so `d.Source` cannot recover ordinary resolving spells there. Source oracle text alone also cannot identify the active subability, X amount, or prevention. The View does not project damage-prevention facts.

**Requested authorization:** coordinate with the owner of protected `rules/stack.go` and authorize a narrowly scoped public target-effect fact contract (also touching `decision/decision.go`), populated from the exact active SA. It should distinguish damage from other effects and unknown from known effective damage; prevention/replacement handling must be specified, with unknown degrading conservatively rather than claiming lethal. Alternatively, controller can explicitly narrow this task to a non-lethal-proving race heuristic, but that would deviate from the brief's warning against life-only lethal inference. I have not made that tradeoff on the controller's behalf.

Per the instruction to report NEEDS_CONTEXT when needing another agent's files, I stopped rather than editing those files or hiding the inference in oracle parsing. The branch still contains the checkpoint's known-bad heuristic: **do not merge it as a finished policy change**.

## Per-file state

- `botpolicy/cast.go`: reconciled the conflicts; shared `cardWorth`, `commandTax`, and `cmdrTaxScale` remain main's implementation, with checkpoint classification confined to casting. `cardWorth` callers are `castScore`, both comparator reads in `chooseTriggerOrder` (`trigger.go`), and the commander-zone branch in `policy.go`. No discard ranking helper was changed.
- `botpolicy/combat.go`, `seat/bot.go`: auto-merge preserved identical `Card.Types` and `Card.Text` filling in BOTH adapter halves, alongside dp2's `Produces`. No new Card fields added by this continuation. The checkpoint comment claiming an adapter-parity test exists is unverified and should be corrected when implementation resumes.
- `view/view.go`: checkpoint's Oracle/Text projection preserved, together with dp2's mana production projection.
- `botpolicy/target.go`, `botpolicy/target_test.go`, `botpolicy/cast_test.go`: inherited unchanged. Existing passing tests do NOT establish the missing effect-aware safety contract.
- The merge commit hook updated seven `TEST_HISTORY.md` files automatically; its output is below. No manual budget edits.
- No protected engine/effect implementation, frozen `legacy.go`, policy rewrite, or golden edited.

## Gates: exact commands and real output

Post-merge, before further implementation:

```sh
GOMEMLIMIT=5GiB go test -p=2 ./botpolicy ./seat ./view ./internal/archtest
```
```text
ok  	github.com/adams-shaun/gorge/botpolicy	0.006s
ok  	github.com/adams-shaun/gorge/seat	1.238s
ok  	github.com/adams-shaun/gorge/view	0.034s
ok  	github.com/adams-shaun/gorge/internal/archtest	0.126s
```

F7 architecture gate PASS. No imports of rules/view/seat were added to botpolicy.

```sh
GOMEMLIMIT=5GiB go test -p=2 ./rules -run 'TestHeads|TestRepoDeckGamesReplayExactly|TestEveryRepoDeckIsFullySupported' -v
```
```text
=== RUN   TestEveryRepoDeckIsFullySupported
    acceptance_test.go:113: ratchet: 0 of 423 distinct cards across the repo decks are not fully supported
--- PASS: TestEveryRepoDeckIsFullySupported (0.25s)
=== RUN   TestRepoDeckGamesReplayExactly
    acceptance_test.go:316: seed 0: 1307 intents, chain ec6fb0fb117db32a, replay OK
    acceptance_test.go:316: seed 1: 828 intents, chain 6d8a0ae8c166a0a7, replay OK
    acceptance_test.go:316: seed 2: 818 intents, chain 3f4c06c53684841e, replay OK
    acceptance_test.go:316: seed 3: 994 intents, chain 8a70aa851d70432a, replay OK
    acceptance_test.go:316: seed 4: 802 intents, chain 16731330d7376cdf, replay OK
--- PASS: TestRepoDeckGamesReplayExactly (0.42s)
=== RUN   TestHeads
    heads_test.go:206: 4 seats: chain head 57afc8f036c996eb, golden 7d178f2232d5e1e7 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:206: 8 seats: chain head dcf2d7af4c9d4f65, golden a5ec8770907c1496 — if this move is intended, update acceptanceHeads and name the cause in the commit body
--- FAIL: TestHeads (0.60s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	1.276s
FAIL
```

```sh
GOMEMLIMIT=5GiB go vet -p=2 ./botpolicy ./seat ./view
```
No output; exit 0.

## Base archive and head evidence

Archived explicit base `0751802152322823156c09cf81d77b9136e632cc` into ignored `.ds4/scratch/dp3c-base`, with a symlink to the existing corpus. No checkout/branch switch or main-tree mutation.

```sh
mkdir -p .ds4/scratch/dp3c-base
git archive 0751802 | tar -x -C .ds4/scratch/dp3c-base
ln -s "$(readlink -f .cards)" .ds4/scratch/dp3c-base/.cards
GOMEMLIMIT=5GiB go -C "$PWD/.ds4/scratch/dp3c-base" test -p=2 ./rules -run TestHeads -v
```
```text
=== RUN   TestHeads
--- PASS: TestHeads (0.62s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.627s
```

`TestHeads` prints only mismatches. The unprinted matching pairs below follow from the successful comparisons with the unchanged `acceptanceHeads` entries:

| seats | base actual / golden | merged checkpoint actual / golden |
|---|---|---|
| 2 | b984373baa683987 / b984373baa683987 | b984373baa683987 / b984373baa683987 |
| 4 | 7d178f2232d5e1e7 / 7d178f2232d5e1e7 | 57afc8f036c996eb / 7d178f2232d5e1e7 |
| 6 | 78c5c443d7e290a6 / 78c5c443d7e290a6 | 78c5c443d7e290a6 / 78c5c443d7e290a6 |
| 8 | a5ec8770907c1496 / a5ec8770907c1496 | dcf2d7af4c9d4f65 / a5ec8770907c1496 |

Partial movement: 4 and 8 only. The delta contains the checkpoint's cast-class bonus and life/commander-clock target scoring. I have NOT isolated which policy branch/card causes each movement, so these are evidence of the unfinished checkpoint, not an attributed golden-regeneration request. Per-card attribution remains owed after the safe rule is implemented. No golden regenerated.

## Merge hook output

```text
pre-commit: measuring test time for changed packages
testtime: botpolicy 0.0s 83 tests budget 5s
testtime: cards 5.6s 73 tests budget 8s
testtime: cmd/botbench 0.0s 34 tests budget 5s
testtime: cmd/gorged 0.3s 17 tests budget 10s
testtime: rules 2.5s 329 tests budget 10s
testtime: seat 1.0s 9 tests budget 5s
testtime: view 0.0s 66 tests budget 5s
pre-commit: staging TEST_HISTORY.md:
  botpolicy/TEST_HISTORY.md
  cards/TEST_HISTORY.md
  cmd/botbench/TEST_HISTORY.md
  cmd/gorged/TEST_HISTORY.md
  rules/TEST_HISTORY.md
  seat/TEST_HISTORY.md
  view/TEST_HISTORY.md
```

## Deferred work / deviations

Stopped for explicit ownership clarification, not for a failed build or budget. No benchmark run: the requested 200-game before/after constructed and commander measurements, priority/cast and target rows, win/coin-flip shares, and byte-identical same-seed benchmark repeat remain owed. Replay itself passes on all five acceptance seeds. No RNG calls were added by the merge. Face-rule regression tests and adapter parity tests still need completion with the agreed effect-fact contract. No broad `go test ./...`, race run, web installation, push, rebase, stash, or source/golden mutation outside ownership was performed.

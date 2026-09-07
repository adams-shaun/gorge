# CR coverage — judge_jebbediah

Reference: Magic: The Gathering Comprehensive Rules, **2026-08-07** revision.
Session: **jj1**, engine base **`63ee22d`**, branch `wt/jj1`.
This ledger was created before the test audit and updated for handoff.

## Resume here

**No rules area is certified conformant.** CR 601 was read in full; the audited
slices are mandatory spell-target feasibility and target/payment ordering.
Their conformance assertions are executable and **known-red**, not fixes.
The rest of casting remains open. Next session: commission the I-2 and
JJ-601-order fixes, then audit CR 601.2b/f–h's real cost and modal choices before
moving to CR 602. Do not repeat this session's source hunt or confuse green
default gates with passing conformance.

The charter and official reference files remain at:

- `/home/sadams/projects/gorge/.superpowers/judge-jebbediah.md`
- `/home/sadams/projects/gorge/.superpowers/research/cr/MagicCompRules-20260807.txt`
- sibling `.pdf` (not re-downloaded, not tracked).

Only the authored ledger, proposal and research index are force-added through
the `.superpowers` ignore. Tests are normally tracked. Source CR files and
Forge material must stay ignored. The charter/ISSUES.md were read from main's
shared research directory because they were absent in this task worktree.

## Coverage map

“Read” means the cited official text was examined. “Audited/tested-red” means
only the named property is covered; it is not permission to skip adjacent rules.
Tests use the compiled corpus, not script-shaped synthetic SAs.

| Rule range | Status / engine paths examined | Tests and findings | Still unexamined |
|---|---|---|---|
| 601.1–1a | Read | No assertion | Terminology / play-land distinction |
| 601.2 introductory paragraph, 601.2a | Read; `commitCast` ordering inspected | Relevant to JJ-601-order; putting the spell on the stack before targeting is itself **correct** under 601.2a | Stack characteristics during proposal; reversal transaction; copies |
| 601.2b | Read; pendingCast structure and cost parser inspected, not conformance-audited | No assertion | Modal timing, X, splice, alternative/additional costs, hybrid/Phyrexian choices |
| 601.2c | **Audited/tested-red, narrow scope**; `legalActions`, `castable`, `askTarget`, `handleTarget` | `TestCR601NoMandatoryCounterCastOnEmptyStack`: I-2; `TestCR601TargetsPrecedeManaPayment`: JJ-601-order | General target legality, target counts, optional/modal/kicked targets, distinctness, same object across separate target clauses, forced targets, target-trigger timing, non-hand casting |
| 601.2d | Read only | No assertion | Division/distribution and minimum allocation |
| 601.2e, 601.6–6a | Read; abort/fizzle source inspected | I-2 loss rather than rewind is a source-supported consequence, **not independently tested here** | General legality recheck, flash conditions and rollback/payment undo |
| 601.2f | Read; `castable`, `ParseCost`, `commitCast` inspected | Cost grammar candidates below, no new assertion | Cost locking, modifiers/order, unsupported nonmana components |
| 601.2g | Read only beyond cast-path inspection | No assertion | Mid-cast mana-ability window, particularly mana sources depending on announced targets |
| 601.2h | **Audited/tested-red, payment timing only**; `payMana` and `commitCast` | JJ-601-order: legal targeted burns pay before targets | Full payment feasibility, payment order, random/library costs, partial payment/undo |
| 601.2i | Read only | No assertion | When the spell becomes cast and cast/target triggers are drained; priority return |
| 601.3–3f | Read; `legalActions` timing/restriction/source walks inspected, not audited | No assertion | Permissions, changing characteristics, conditional flash, face-down cards |
| 601.4–5 | Read only | Scope limit on I-2 oracle: do not infer all future target/cost feasibility from current zones | Choices dependent on later choices |
| 601.7–7b, 601.8 | Read only | No assertion | Opponent choices and costs of already-stacked spells |
| 115.5 | Read exact rule | Self-target exclusion supports empty-stack oracle; existing target test uses an **older 114.4 citation** | No new standalone test; existing test not changed |
| 733.1–2 | Read via CR 601 cross-reference | Reversal rationale for I-2; no independent test | Exceptions to reversal and retained priority |
| 602 activation | Unread | No conformance assertion this session | Entire area; shared commitCast targeting order is a lead, not a measured result |
| 603 triggers | Unread | Trigger/resume fields inspected for observability only | Entire area |
| 608 resolution | Unread CR text; source/issue evidence inspected for proposal | I-1 remains open, not reproduced | Reproduce at deployed revision and preserve evidence before diagnosis |
| 609–614 effects/replacement | Unread CR text; `emit`/`applyReplacements` inspected for proposal | No conformance assertion | Entire area; later include CR 616 selection/order |
| 704 / keyword actions | Unread | None | Entire area |
| 506–511 combat | Unread | None | Entire area |
| 800s multiplayer | Unread | I-1 departure/continuation hazard only read as issue evidence | Entire area, especially departure with an outstanding answer |

## Findings and disclosure at base

### I-2 — mandatory spell-target cast offered with an empty stack

**Not in AGENTS.md's Known approximations table.** Already recorded in shared
`.superpowers/ISSUES.md` as I-2, so not a newly discovered issue; it now has a
real-corpus, decision-loop conformance test. This is silent with respect to the
engine's approximation ledger, not silent with respect to the issue tracker.

`legalActions` checks payment and timing, not target feasibility. The invariant
examines every visited priority decision in the acceptance configurations at
`Seed:42`, `Mulligans:1`, with `newTestBot(7)`. It narrows the oracle to compiled
Counter SAs with `TargetType=Spell`, `ValidTgts=Card`, and absent
TargetMin/TargetMax/TgtZone (the unconditional default target shape). When the
stack is empty, no existing spell can be targeted; CR 115.5 excludes the spell
being proposed. The cast option must not be in an interface defined as legal
actions. The CR allows proposal followed by reversal, **not** payment and
permanent card loss on failure to complete CR 601.2c.

Failure examples include both base and alternative-cost offers. The test
fails at the first violating decision in each seat configuration, so these
runs are **not** complete game audits. On a fixed engine it must finish the
games and still examine candidate cards. The vacuity guard counts inspected
hand-card/boundary pairs, not offending options, so removing illegal options
does not disable the guard. It does not cover graveyard/command casts, general
battlefield-target feasibility, or targets dependent on later casting choices.

`TestCounterspellWithOnlyItselfOnStackFizzles` in
`rules/stack_target_spell_test.go` currently pins the incorrect card-loss
behaviour using a synthetic fixture. Its claim that casting with no target
“must fizzle” conflicts with CR 601/733. The eventual fix owner must change that
test, not retain the divergence to keep it green. jj1 did not change it.

### JJ-601-order — targets selected after mana is paid on legal spells

**Not in AGENTS.md's Known approximations table; new finding this session.**
The “as this enters” row concerns ETB choices, not ordinary spell targeting;
the mana-production/restriction rows do not disclose payment-before-targets.

`TestCR601TargetsPrecedeManaPayment` casts real compiled Lightning Bolt, Shock
and Incinerate through an actual priority intent. Living player targets are
available, so I-2 is not responsible. The target decision is outstanding and
no TargetsChosen has been recorded, but the mana pool has decreased. CR 601.2
requires the listed steps in order, with **601.2c before 601.2h**.

The test uses deterministic repeated-card decks to reach the boundary without
an acceptance bot choosing a different spell; all setup mutation after genesis
is emitted, not a direct Game write. It asserts a nonempty real target decision
for the selected source and a payable cast offer. No nonmana-cost ordering or
post-target payment amount is claimed tested. The fixture's card multiplicity
is not a claim about Constructed deck legality.

`commitCast` clears pendingCast, pays mana, applies selected cost moves, then
puts the card on the stack and asks targets. This is broader than I-2's missing
preflight. A target-feasibility gate alone will not fix this ordering defect.

### Leads, not findings with new executable proof

- `rules/mana.go` explicitly comments that hybrid and Phyrexian costs become
  generic mana. That is disclosed in source, but no corresponding Known
  approximations row was found in the supplied AGENTS.md table. Audit real
  compiled Dismember/Gitaxian Probe and the announcement/payment path next;
  do not infer measured corpus reach from that comment.
- Modal selection is implemented in mid-resolution machinery. CR 601.2b
  requires modal spell choices on casting; distinguish the choice's timing from
  the already-documented no-ask-host fallback before assigning a new defect.
- `castable`'s greedy sacrifice reservation is explicitly described as
  conservatively withholding some payable casts. Cost/source comments are not
  CR conformance proof. A real-card example is still owed.
- No corpus-reach counts were measured. No AGENTS.md rows were edited or
  silently reclassified as approved approximations.

## Reproduction / baseline proof

Tests live in `rules/cr601_conformance_test.go`. **Both are opt-in**:

```sh
GORGE_CR_CONFORMANCE=1 GOMEMLIMIT=5GiB go test -p=2 ./rules -run '^TestCR601' -v
```

Default tests print explicit skips naming the unfixed finding and stating that
it is NOT an AGENTS.md approximation. This is the session's explicit compromise
between “do not change engine”, “land failing assertions” and “default gates
must pass”. It is not an assertion of the broken behaviour. Remove each guard
when its fix lands; until then run the command above in every judge/fix audit.
The controller should decide whether to put this known-red lane into CI.

A baseline archive was created and tested with exactly the new test file:

```sh
base=$(mktemp -d /tmp/gorge-jj1-base-XXXXXX)
git archive 63ee22d | tar -x -C "$base"
ln -s /home/sadams/projects/gorge/.cards "$base/.cards"
git -C "$base" init -q
git -C "$base" add .
git -C "$base" -c user.name='Conformance baseline' -c user.email='baseline@localhost' \
  commit -qm 'test: initialise archived 63ee22d for corpus root discovery'
cp rules/cr601_conformance_test.go "$base/rules/"
GORGE_CR_CONFORMANCE=1 GOMEMLIMIT=5GiB go -C "$base" test -p=2 ./rules -run '^TestCR601' -v
```

Actual retained archive: `/tmp/gorge-jj1-base-eT3JH6` (temporary convenience, not
the durable evidence). The git directory enabled corpus root discovery; the
corpus symlink enabled actual execution. Exit **1**, assertion failures, not
compilation failures or skipped corpus tests. Baseline failure messages:

```text
2 seats, intent 204: priority decision seq 1140 offered 95 ("Cast Daze (alternative cost)") with an empty stack — CR 601.2c: a required spell target must be available; CR 115.5 forbids targeting itself
4 seats, intent 46: priority decision seq 253 offered 93 ("Cast Force of Will (alternative cost)") with an empty stack — CR 601.2c: a required spell target must be available; CR 115.5 forbids targeting itself
6 seats, intent 916: priority decision seq 4185 offered 288 ("Cast Mana Leak") with an empty stack — CR 601.2c: a required spell target must be available; CR 115.5 forbids targeting itself
8 seats, intent 86: priority decision seq 436 offered 95 ("Cast Daze (alternative cost)") with an empty stack — CR 601.2c: a required spell target must be available; CR 115.5 forbids targeting itself
target decision seq 29 for source 5 ("Lightning Bolt") is unanswered but mana changed [0 0 0 3 0 0] -> [0 0 0 2 0 0] — CR 601.2c/601.2h: choose targets before paying costs
target decision seq 29 for source 5 ("Shock") is unanswered but mana changed [0 0 0 3 0 0] -> [0 0 0 2 0 0] — CR 601.2c/601.2h: choose targets before paying costs
target decision seq 29 for source 5 ("Incinerate") is unanswered but mana changed [0 0 0 3 0 0] -> [0 0 0 1 0 0] — CR 601.2c/601.2h: choose targets before paying costs
```

Worktree execution produced the same substantive failures. Full outputs and
gates are pasted in `.ds4/task-jj1-report.md`; durable failure evidence is above.

## Reference lookup and verification

The executed rule-number locator (system `/usr/bin/grep`, not the shell's
ignore-aware alias) was:

```sh
/usr/bin/grep -n -E '^ *601\.' /home/sadams/projects/gorge/.superpowers/research/cr/MagicCompRules-20260807.txt
/usr/bin/grep -n -E '^ *115\.5|^ *733\.[12]' /home/sadams/projects/gorge/.superpowers/research/cr/MagicCompRules-20260807.txt
```

The complete CR 601 section and CR 733.1–2 were read with the file reader after
locating them; citations are rule numbers, not pages. No `.cards` grep/count was
used. Revision-sensitive correction: self-target prohibition is CR **115.5**
here, despite the older citation in existing engine comments/tests.

Default `GOMEMLIMIT=5GiB go test -p=2 ./rules` passed. The single whole-tree run
`GOMEMLIMIT=5GiB go test -p=2 -json ./...` passed, including TestHeads,
TestRepoDeckGamesReplayExactly and TestEveryRepoDeckIsFullySupported; new CR
checks skipped by design. `go run ./cmd/gentypes -check`, rules vet and gofmt
were clean. No heads or engine source changed. Ordinary gates omit `-count=1`
per the later fleet cache instruction. The opt-in baseline used a fresh archive.

## Observability handoff

See [observability-proposal.md](observability-proposal.md). It proposes a private,
immutable boundary snapshot and parallel branch trace with no events.Kind/Event
change. It distinguishes attempted vs persisted moves, captures continuation
and cost/target stage provenance, and requires a versioned configuration/IR
manifest alongside event/intent files. I-1 has not been resolved by this study.

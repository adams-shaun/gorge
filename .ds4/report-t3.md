# Report — game-long damage-by-source provenance (round t3, fix round)

Ticket: `agent-20260923T114033Z-a57ee463`
Branch: `wt/agent-20260923T114033Z-a57ee463`
Worktree: `/home/sadams/projects/gorge/.worktrees/agent-20260923T114033Z-a57ee463`
HEAD before this round: `b025710d4` (the reviewed t2/mrg1 commit)
`.cards` present: **yes**, a real symlink to `/home/sadams/projects/gorge/.cards`.

## What this round was

`findings-t3.md` carried exactly ONE MAJOR and no re-verification list:

- **[MAJOR]** `.ds4/report-mrg1.md` and `.ds4/report-t1.md` had had 6,605 and
  3,389 lines removed respectively by prior commits on this branch, replacing
  the tracked shared report files' accumulated history with this ticket's
  round text. Restore the removed history byte-for-byte and make this round's
  report additive.

Every other item in `findings-t3.md` was a break attempt that HELD:
event encoding/replay, Player 0 recipient vs missing recipient,
prevented/redirected/negative damage, unbound/unknown refs and negation,
and the two regression goldens (`internal/archtest`, `cmd/botbench`), which
the reviewer ran green (held on inspection / cached).

## The fix

Confirmed the finding, and found the class extends to a third file the
finding did not name: `.ds4/report-t2.md` also deleted main's accumulated
history (1541 lines, 1329 deletions vs `main`). All three shared report files
on this branch had been written as a *replacement* rather than an *append*.
The evaluation of this worktree at `HEAD` showed `.ds4/report-t1.md` at 221
lines against `main`'s 3750, `.ds4/report-t2.md` at 288 against `main`'s
1405, and `.ds4/report-mrg1.md` at 273 against `main`'s 138.

Restored all three using the established shared-report-file preserve
convention (the separator shape already present throughout `main`'s own
`report-t1.md`, and the shape commit `41c1348fe` used on this same branch):

```
<this ticket's report for that lane>

---

# Reports appended below are from other tickets on the shared report file (preserved verbatim from main):

<main's content for that file, byte-exact>
```

Verification that the preserved half is byte-exact (sha256 of `main`'s file
equals the sha256 of the tail of the union, after the 6-line separator):

```
$ git show main:.ds4/report-t1.md | sha256sum
b47a922b29c4a0a60b1294f1bcea5b32ac4d7adee3414a5187768293986ba4eb  -

$ tail -n +227 .ds4/report-t1.md | sha256sum
b47a922b29c4a0a60b1294f1bcea5b32ac4d7adee3414a5187768293986ba4eb  -

$ git diff --numstat main -- .ds4/report-t1.md .ds4/report-t2.md .ds4/report-mrg1.md
226     0       .ds4/report-t1.md
293     0       .ds4/report-t2.md
278     0       .ds4/report-mrg1.md
```

Additions-only against `main` for all three files: zero deletions.

`report-t2.md`'s byte-exactness was checked at `tail -n +294` (288 lines of
this ticket's report + the 6-line separator) and `report-mrg1.md` at
`tail -n +279` (273 + 6); each `cmp`s clean against `main`'s file.

## Findings coverage

| finding | status |
|---|---|
| [MAJOR] report history deleted from `report-mrg1.md` / `report-t1.md` | **FIXED** — restored byte-exact, plus `report-t2.md`, same class |

## Gates (unchanged code since the reviewed `b025710d4`; report-only diff)

The diff this round is `.ds4/*.md` only — no Go source, test, `cards/` or
`AGENTS.md` change. The gates below are therefore cached; they are re-run to
prove the tree still reports green, not to re-validate the code.

```text
$ go test ./internal/archtest/ 2>&1 | tail -15
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)

$ go test -run 'TestDamageAllValidPlayers|TestContextWordPredicates|TestUnimplementedPredicateFailsClosed' ./effects/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/effects	(cached)

$ go test -run 'TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets' ./rules/ 2>&1 | tail -20
ok  	github.com/adams-shaun/gorge/rules	(cached)

$ go test -run TestGenerateCommittedFixture ./cmd/repro/ 2>&1 | tail -10
ok  	github.com/adams-shaun/gorge/cmd/repro	(cached)
```

## Fails without the fix

Not applicable to this report-restoration round: the fix is a `.ds4/` text
restoration, and the finding's own test is the `git diff --numstat` above —
before the fix it read `0 6646`, `0 3389` and `0 1329` deletions; after it,
`0` deletions on all three files. The code behaviour under review was verified
in round t2 and is untouched here.

## Issues

No new defect. The one item found this round is that `report-t2.md` carried
the same history-deletion defect as the two files the finding named; it is
fixed here rather than filed, since it is the same class the finding asked to
be fixed. No code, card or engine defect was found.

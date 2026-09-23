# Split-card alternate casting and Fuse — verification report

The requested behavior is already present in this branch's committed history; this round made no source changes. The implementation is in `rules/legal.go`, `rules/cast.go`, `rules/split.go`, and `rules/stack.go`, with regression coverage in `rules/split_card_test.go`. The original implementation commit is `b8347d4b` (`feat(rules): cast a Split card's alternate half and its fused cast`); follow-up correctness fixes are also ancestors of the current branch. `.cards/` exists and resolves to `/home/sadams/projects/gorge/.cards`.

The implementation offers the second face of non-Room, non-Aftermath Split cards; Fuse offers a fused cast paying both halves' costs. Rooms and Aftermath remain handled by their existing separate paths. `kw:Fuse` is registered. The current tests cover split face offers, Fuse cost/targeting/resolution, and the keyword registration. The requested specific `Coward // Killer` and `Gallifrey Falls // No More` names/tests are not present in the current corpus or repo deck files; the current regression tests use corpus cards Wear // Tear and others instead. Corpus measurements confirmed 128 `AlternateMode:Split` files and 17 `^K:Fuse` files.

## Verification

- `go test -run 'TestSplit' ./rules/`:
  ```text
  ok   github.com/adams-shaun/gorge/rules  0.603s
  ```
- `go test ./internal/archtest/ 2>&1 | tail -15`:
  ```text
  ok   github.com/adams-shaun/gorge/internal/archtest  3.649s
  ```
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5`:
  ```text
  ok   github.com/adams-shaun/gorge/cmd/botbench  1.397s
  ```
- `gofmt -l rules/split.go rules/legal.go rules/cast.go rules/stack.go rules/split_card_test.go; go run ./cmd/gentypes -check`: no output, exit 0.

No `TestHeads`, acceptance, `make report`, or conformance run was performed; these are daemon gates. No tests were added in this round, so a remove-the-fix failure demonstration is not applicable (`## Fails without the fix`: no new tests or implementation hunk to revert).

The worktree began clean at `bad06ce749438c98dc3db3f3dbc5365c89946feb`, which is also `main`; no rebase was run because the higher-priority workspace instructions prohibit running `git rebase`. No commit was created because there are no source changes to commit.

## Issues

No unfixed code defect was found within scope. The brief's named card fixtures are absent from this repository/corpus; the extant tests pin the same feature using other corpus split cards. The Fuse rules citation in the task title/body says CR 702.36; Fuse is CR 702.101 (as the implementation and tests correctly document).

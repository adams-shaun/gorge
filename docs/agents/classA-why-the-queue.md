# Why class A reaches the merge queue

Follow-up to the red-batch census in
[`red-batches-20261128.md`](https://github.com/adams-shaun/gorge/blob/hotfix/speed-S5/docs/agents/red-batches-20261128.md)
(open PR #6024, branch `hotfix/speed-S5`). That census classified 20 red
`ci` merge-queue runs and found **10 class A** ("real break carried by one
PR"). This note asks one question for each of those 10: **why did the breaking
PR's own checks pass before it entered the queue?**

This file is a separate companion to the census so it does not conflict with
#6024, which adds the census itself to `main`.

## Method

For each class-A run: `gh run view RUN --json jobs` for the failing jobs and
their steps, `gh run view RUN --log-failed` for the failing test, then the
carrier PR from the census, and `gh pr checks` / `gh run view --json jobs` on
**that PR's own `ci` run** for the same commit's check list. The failing test's
gate is read from `.github/workflows/ci.yml`'s job conditions. All read-only;
no `gh api`.

Only these jobs exist on a **plain pull request** (not `mergify/merge-queue/*`):

| Job | On a plain PR | Note |
|---|---|---|
| `fast` | runs, `-short`, `test-targets.sh list` selection | the only Go-test gate |
| `lint` | runs, but skips `go vet ./...` and "A fresh clone builds…" | fresh-clone is queue/push only |
| `web-unit` | runs | vitest, not the Playwright e2e |
| `ci-ok` | passes with `heavy=skipped` | two-step CI |
| `prepare`, `test (0..5)`, `examples`, `wire-matrix`, `dirac-slices`, `dirac-repros`, `oracle-sentinels`, `browser` | **skipped** | condition `github.event_name != 'pull_request' \|\| startsWith(head_ref,'mergify/merge-queue/')` |

So a break whose only gate is a heavy job cannot fail the PR's own CI; and a
break whose gate `fast` *could* run but did not select also passes. Those are
the only two ways class A gets through, and they split exactly evenly below.

## Class table

| Run id | PR (carrier) | Class | Evidence |
|---|---|---|---|
| 46153413812 | #5984 | **1** narrow selection | `fast` selection (run 46154148577) lists `whole ./dirac` … but **not** `./dirac/crcoverage`; `test-targets.py`'s `SOURCE_READERS` maps only `dirac/kgen` → `crcoverage`, and crcoverage's test binary imports no gorge package, so a change to `dirac/chars_gen.go` never selects it. The test's other gates (`lint` fresh-clone, `test (0)`) are also PR-skipped. |
| 46152946419 | #5984 | **1** narrow selection | Same carrier and same selection gap; batch `#5984 + #5986`. |
| 46121803294 | #5862 | **4** skipped on pull_request | e2e `web/e2e/smoke.spec.ts:958` "a refused deck is disabled in the picker"; `browser` is merge-queue-only and #5862's own check list shows `browser skipping`. |
| 46121257901 | #5862 | **4** skipped on pull_request | Same e2e smoke (carrier #5862 was already on main); #5862's own `browser` is `skipping`. `test (4)` pod timeout is the concurrent class C. |
| 46120713214 | #5862 | **4** skipped on pull_request | Same e2e smoke; batch includes #5862, whose own `browser` was `skipping`. |
| 46119791110 | #5861 | **1** narrow selection | `TestEffectStaticCensus` (engine/rules) parses `docs/agents/known-approximations/effect-modes.md`; #5861 is docs-only and `test_targets.py` drops every `.md` (`if path.endswith(".md"): continue`), so `fast` selected nothing and passed. The same run also failed the e2e smoke (class 4 for #5862). |
| 46117688048 | #5845 | **4** skipped on pull_request | `TestReproManifest4/mutate_merge_object_count` runs only in `dirac-repros` (merge-queue-only); #5845's own check list shows `dirac-repros skipping`, and `fast` sets `CI_FAST_SKIP_ORACLE_REPLAY=1` so the replay never runs on a PR either. |
| 46117264984 | #5845 | **4** skipped on pull_request | Same carrier, same `dirac-repros skipping`; batch `#5845 + #5846`. |
| 46111358109 | #5807 | **1** narrow selection | `TestCRDeviation730_2d` (engine/rules) pins the old CR 730.2d behaviour; #5807 changed the behaviour but its first head left the pin. engine/rules is `HEAVY`, so `fast` ran only the changed test files' named tests + `CENSUS`; the pin test matches neither. `test (3)` is merge-queue-only. |
| 46110843281 | #5807 | **1** narrow selection | Same carrier and same `HEAVY` named+census gap; batch `#5803 … #5817`. |

### Counts

| Class | Count | Runs |
|---|---|---|
| 1 — narrow selection | **5** | 46153413812, 46152946419, 46119791110, 46111358109, 46110843281 |
| 4 — skipped on pull_request | **5** | 46121803294, 46121257901, 46120713214, 46117688048, 46117264984 |
| 2 — stale base | 0 | — |
| 3 — prior-skip | 0 | — |
| 5 — checks changed after queueing | 0 | — |
| 6 — other | 0 | — |

No case was stale base (class 2): each carrier's break reproduces on its own
base, and no batch tested a sha the PR never ran. No case was prior-skip
(class 3): `prior` is `if: github.event_name == 'push'`, and every one of the
10 runs is a `pull_request` event whose `ci-ok` log shows `prior=` (empty) and
`fast=skipped` — the heavy stage ran in the batch, as intended. No case was
"checks changed after queueing" (class 5).

## The single highest-value change

The largest single lever is **the class-4 half: run the affected heavy work in
the PR's own `fast` stage, path-gated** — which prevents 5 of the 10 outright,
and would have prevented all 10 if the heavy stage simply ran on the PR. That is
a design/cost call (it changes what runs on `pull_request` and adds steps/jobs to
`ci.yml`), so it is **proposals-only**; nothing is implemented here.

- For the class-4 runs the concrete proposal is: when a diff touches `web/`
  (or `web/src/components/PlayVsBot.svelte` specifically) run the affected
  Playwright smoke spec on the PR; when it touches
  `dirac/oracle/testdata/repro/` run the changed recording's `TestReproManifest*`
  replay on the PR (drop `CI_FAST_SKIP_ORACLE_REPLAY` for a changed recording,
  which is already what the guards do for the table tests).
- The other 5 (class 1) are **not** one change: they are three distinct
  selection-model gaps in `scripts/test_targets.py`, each a small, separate
  mapping fix:
  1. `SOURCE_READERS` maps `dirac/crcoverage` to `dirac/kgen` only, but the
     coverage tables cite all of `dirac/*.go` (chars_gen.go, static_*.go, …).
     A change to `dirac/chars_gen.go` should select `./dirac/crcoverage`
     (it is a ~2 s package). This alone would have caught #5984's two runs.
  2. `HEAVY` `./engine/rules` runs only the changed test files' named tests plus
     `CENSUS`; the CR-deviation pin tests (`TestCRDeviation*, cr_deviation_pins_test.go`)
     are neither. A change to a non-test engine file a rule test pins should run
     those pins. This would have caught #5807's two runs.
  3. Markdown is "never an input", but `TestEffectStaticCensus` parses
     `docs/agents/known-approximations/*.md`. A change under that directory
     should select `./engine/rules`' census test. This would have caught
     #5861's run.

If forced to name one change today, it is the class-4 path-gated PR heavy step
(5 runs, and the structural reason all 10 were first seen in the queue). The
three selection fixes are the cheap follow-ups that remove the other half.

### Why #6029 and #6013 do not cover this

- **#6029** adds a path-gated check of the dirac *walk / types-cache invariant*
  (the quiet-proof and `TestTyCacheExcludesNonObjectSources` lines) to `fast`.
  That is the class-B guard: it never runs `dirac/crcoverage`, the CR-deviation
  pins, the `known-approximations` census, the oracle repro replays, or the
  Playwright smoke. It would not have caught any of these 10.
- **#6013** adds a bounded retry for a vanishing Go **build-cache** entry. That
  is the class-C flake guard; every one of the 10 is a deterministic break, not a
  cache miss, so the retry changes nothing for them.

## One latent defect worth filing separately (not the cause here)

`ci-ok`'s "Read the tree this pull request passed" step uploads
`ci-passed-tree-<tree>` for **any** `pull_request` whose `ci-ok` passed,
including a plain PR where the heavy jobs were `skipped`. `prior` then treats
that tree as tested on the push to `main`. In a fast-forward merge (main is an
ancestor of the PR head) the PR head tree equals the merge tree, so in principle
a plain PR's narrow `fast` pass can mark a tree "tested" that the heavy stage
never ran. In these 10 reds it did **not** fire: the red runs are all
`pull_request` (queue-batch) events with `prior=` empty, and the one main push
that skipped heavy (46122240320) matched a tree a queue batch had actually
passed. The one-line hardening — require
`startsWith(github.head_ref, 'mergify/merge-queue/')` before recording the
tree — is additive (`ci.yml`), but it is not the cause of class A and is left
out of this PR to avoid a three-way conflict with #6013 and #6029.

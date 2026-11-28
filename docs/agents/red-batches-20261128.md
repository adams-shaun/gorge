# Red merge-queue batches — 2026-11-28

Census for operator act-file item **S5**: every red Mergify merge batch costs a
whole re-run, so classify the reds by cause and give each cause class one guard.

## Scope and method

- The merge-queue universe is `ci` workflow runs whose head branch starts
  `mergify/merge-queue/` (`gh run list -w ci`), plus any red `queue-precheck`
  run. There were **31** red merge-queue runs in the last 500 `ci` runs (back to
  `2026-11-28T01:05Z`); this document classifies the **most recent 20**. No red
  `queue-precheck` run appeared in the window.
- For each run: `gh run view RUN --json jobs` for the failing jobs, then
  `gh run view RUN --log-failed` for the failing test or error. Batch composition
  comes from each run's `displayTitle` (`merge queue: checking …`), which names
  the PRs the batch tested.
- Cause is taken from the log and from git history (fix/revert commits), **not**
  from the check name.
- **No box or host names appear here** (a self-hosted runner, not its name).

## Class table

Classes:

- **A — real break, one PR** (behaviour changed without its pin/doc/register).
- **B — real break, one PR, perf-invariant** (a `perf(dirac)` PR relaxed a
  quiet-proof or cache invariant). New class, recurring.
- **C — flake** (vanishing Go build-cache entry, or a heavy-test timeout).
- **D — unreadable** (logs withheld/empty at census time; not guessed).
- **E — semantic conflict** between two batched PRs.
- **F — stale gate / queue-precheck refusing on a red main.**

| Run id | Head sha | Failing job(s) | Failing test / symptom | Class |
|---|---|---|---|---|
| 46156810528 | 5d9c37c712 | dirac-slices, test (0), test (5) | quiet proof "the walk changed the State" (`cast_quietseat_test.go:40`) + "table is not cached with Duplicant excluded" (`static_typecache_test.go:140`) + edh `projection NStack` divergence; test (5) also hit a build-cache miss | **B** (carrier #5994); test (5) a concurrent C |
| 46155595807 | 7d210e4497 | dirac-slices | edh: 1 divergent, class `projection NStack`, implicates Hearthhull | **B** (batch was "#5994 on main", alone) |
| 46154479584 | 7e2616f1c5 | dirac-slices, test (0) | edh `projection NStack` divergence + `TestQuietProofEDH` | **B** (batch #5991+#5995+#5994+#5986) |
| 46153413812 | 6c0e9845e9 | lint, test (0) | `TestCRCoverageIsCurrent` (CR coverage map cites a symbol moved out of `chars_gen.go`) | **A** (carrier #5984; fixed in-PR by e23e066f58) |
| 46152946419 | 22aef17aad | lint, test (0) | `TestCRCoverageIsCurrent` | **A** (batch #5984+#5986) |
| 46140183894 | a8ba7dd252 | lint | fresh-clone `panic: test timed out after 2m0s`; `TestHotAccessorsInline`, `TestNoKernelLocalEscapes` at 1m24s | **C** (timeout under load) |
| 46129867243 | 643cc2aea5 | test (2), test (4) | `could not import …: no such file or directory` (Go build cache) across many packages | **C** (vanishing build-cache entry) |
| 46128867881 | 3c02e300d2 | test (4) | `TestCreatePodDiracSeatsFour` 120.06s, "the pod has no end after 173 answers" | **C** (long-game timeout; same test passed in adjacent batches) |
| 46127056852 | 91e97dacad | lint | `could not import …: no such file or directory` (Go build cache) | **C** (vanishing build-cache entry) |
| 46122521600 | 1c4682d0b1 | web-unit | `eslint` step failed; log withheld (0 lines) | **D** (unreadable) |
| 46121803294 | 74ec348c36 | browser | e2e smoke "a refused deck is disabled in the picker": expected "Giada, Font of Hope", element not found | **A** (brittle e2e ordering; fixed c5646d9b7b / bbed1bdc65) |
| 46121257901 | 84fd5e2300 | browser, test (4) | same e2e smoke + `TestCreatePodDiracSeatsFour` timeout | **A** (e2e smoke) |
| 46120713214 | 0f3214f27e | browser | same e2e smoke failure | **A** (e2e smoke) |
| 46119791110 | 453bfdff4a | browser, test (4) | same e2e smoke + `TestEffectStaticCensus` | **A** (e2e smoke; census fixed 318f8e35a8) |
| 46117688048 | f5ade56faf | dirac-repros (1) | `TestReproManifest4/mutate_merge_object_count` | **A** (mutate lowering repro) |
| 46117264984 | ef398cffe7 | dirac-repros (1) | `TestReproManifest4/mutate_merge_object_count` | **A** (mutate lowering repro) |
| 46112371163 | 8a1cd338cc | lint, prepare (spotci) | both jobs zero steps, no logs, fail in <25s | **D** (logs withheld; timing suggests infra) |
| 46112155917 | 644d8ea339 | prepare (spotci), lint | logs withheld (0 lines) | **D** (unreadable) |
| 46111358109 | fc8e11b12f | test (3) | `TestCRDeviation730_2d` | **A** (rules change without dropping its pin; fixed 97d409aacd) |
| 46110843281 | e3c1e8085b | test (3) | `TestCRDeviation730_2d` | **A** (same, fixed 97d409aacd) |

### Counts by class

| Class | Count | Runs |
|---|---|---|
| A — real break, one PR | 10 | 46153413812, 46152946419, 46121803294, 46121257901, 46120713214, 46119791110, 46117688048, 46117264984, 46111358109, 46110843281 |
| B — real break, one PR, perf-invariant | 3 | 46156810528, 46155595807, 46154479584 |
| C — flake (build cache / timeout) | 4 | 46140183894, 46129867243, 46128867881, 46127056852 |
| D — unreadable / skipped | 3 | 46122521600, 46112371163, 46112155917 |
| E — semantic conflict | 0 | — |
| F — stale gate / queue-precheck | 0 | — |

## Class B: which PR most likely carries it

The newest red, 46156810528, tested `#6013 + #5994 + #6003 + #6005 + #6008 on
main (ac26250)` and failed with exactly two invariant lines — the quiet proof
(`cast_quietseat_test.go:40`) and the types cache (`static_typecache_test.go:140`)
— plus the edh `projection NStack` divergence. `#6013` is CI-scripts only, and
`#6003`/#6005/#6008 touch an altar lockstep test, a refusal row and the IR ref
atoms; none reach the payment walk or the slices. **`#5994` is the carrier**: its
diff rewrites `dirac/mana_window.go` (`windowDryWalk` and
`paymentStrandedPlan`), touches `dirac/oracle/pools.go`, and re-cuts the
`edh`, `hosted-cmdr` and `standard-meta` slice pairs (`pairs.tsv` regenerated,
~2.6k lines). That is exactly the walk the quiet proof checks and the slices
that run the edh divergence.

The attribution is confirmed by batch titles: 46155595807 tested **`#5994`
alone** ("stacked on #5991 and #5995") and still hit the edh divergence, and
46154479584 tested `#5991 + #5995 + #5994 + #5986`. The same quiet-proof line was
the culprit in the earlier red batch that ejected `#5986`, so this is the **same
class recurring from a second perf PR**: a `perf(dirac)` change that relaxes a
quiet-proof/cache invariant the heavy shards assert. It is not a Mergify
condition problem — the invariant is test-side.

## One guard per class (keep every gate and test)

- **A — real break, one PR.** Run the affected ratchet/pin/doc/repro/heavy tests
  in the PR's **own** CI before it can enter the queue, keyed on changed paths:
  CR-coverage (`dirac/crcoverage`), approximation register
  (`known-approximations`), CR deviation pins, `dirac-repros`, and the affected
  e2e specs. Today the fast PR stage does not run heavy shards, so the queue is
  the first to see them. Where: a path-filtered step in the `fast` job of
  `.github/workflows/ci.yml`, driven by `scripts/` (e.g. the existing shard
  weights from `scripts/ci-fast.sh`) — split and pre-check, never drop.
- **B — perf-invariant break.** Add the quiet-proof (`TestQuietProofEDH`) and
  types-cache (`TestTyCacheExcludesNonObjectSources`) assertions to the PR fast
  stage whenever a diff touches `dirac/mana_window.go`, `dirac/state*.go`, the
  dirac cache files, or the slice tarballs under `dirac/oracle/testdata/slices/`.
  This is a **test-side/CI check on the invariant, not a Mergify condition**.
  Where: a path-gated step in the `fast` job of `.github/workflows/ci.yml` (a
  small `scripts/pr-dirac-invariant.sh`), on top of the existing tests.
- **C — flake (build cache / timeout).** The build-cache part **already has a
  guard**: #6013's bounded cache-miss retry (`scripts/ci-retry-cache.sh`) and the
  existing `scripts/ci-infra-retry.sh` wrapper — do not add a duplicate. The
  timeout flake (`TestCreatePodDiracSeatsFour`, the fresh-clone accessor tests) has
  no guard; route those two tests through the same bounded
  `scripts/ci-infra-retry.sh` retry so one timeout does not re-run a whole batch.
- **D — unreadable.** Capture what a job writes unconditionally, so a red is
  never silent: #6059 adds an `if: always()` artifact upload to the `fast`,
  `lint`, `test`, `web-unit`, `dirac-slices` and `dirac-repros` jobs of
  `.github/workflows/ci.yml`, with a 7-day retention. What it **covers** is the
  jobs that already tee their full output to a workspace log — the `test` shards
  (`test-shard.log`, the retried attempt as `.1`), `dirac-slices` (`slices.log`)
  and `dirac-repros` (`repro-*.log`) — plus `fast`'s selected-test list. What it
  **does not** is the way class D actually bit here: a job's own console log
  cannot be read from inside that job, and a job that never starts a step —
  46112371163 and 46112155917 — runs no upload step at all; 46122521600's eslint
  output was withheld by GitHub's log service, which no step can re-read. The one
  known follow-up that would close the eslint instance is tee-wrapping the `lint`
  and `web-unit` commands under `set -o pipefail`; **not done**, because it edits
  the commands that decide pass/fail.
- **E — semantic conflict (0 instances here).** No instance in this census. The
  cheap preventive guard if one appears: a combined `go build ./...` (and `go vet`)
  of the batch head tree as the first heavy step, so a duplicate declaration or
  compile interaction fails in seconds instead of a full shard re-run. Where:
  first step of the heavy stage in `.github/workflows/ci.yml`.
- **F — stale gate / queue-precheck (0 instances here).** No red `queue-precheck`
  run and no stale-gate refusal in the window, so no new guard is proposed.

## Notes

- **Could not read (class D):** 46122521600 (web-unit eslint, no log lines),
  46112371163 and 46112155917 (prepare/spotci + lint, zero-step jobs and no
  logs). Recorded, not guessed.
- **Whole-oracle FYI:** run 46152492514 (`main` 411a225bb5, pre-#6006) has one
  red shard, `oracle (1)`, with logs withheld until it completes. The ward red it
  likely reports was already fixed by #6006 (merged 14:51:04), so it is **not**
  counted as a new class.

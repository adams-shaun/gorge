# Agent seats available for gorge work

*Verified 2026-09-07. Re-verify with `pi --list-models` and the `enabledModels`
key in `~/.pi/agent/settings.json` before relying on it — this file records what
was dispatchable, not what exists.*

Implementation work is offloaded to agent seats; the expensive model is the
reviewer and the gate, not the implementer. This file records **which seats can
actually be dispatched**, because `pi --list-models` prints a large catalogue
(huggingface, openai, anthropic, deepseek…) of which only a small subset is
enabled. Dispatching a model outside that set fails at launch.

## Dispatchable through `pi-agent`

`enabledModels` in `~/.pi/agent/settings.json` is the authority:

| Seat | `--provider` / `--model` | Context / max out | Cost | Use for |
|---|---|---|---|---|
| **Local (default)** | `bm-llms` / `DeepSeek-V4-Flash` | 262K / 32K | free | all ordinary task work |
| Codex | `openai-codex` / `gpt-6-astra` | 272K / 128K | ChatGPT plan | escalation |
| Codex | `openai-codex` / `gpt-5.6-sol` | 272K / 128K | ChatGPT plan | escalation |
| Codex | `openai-codex` / `gpt-5.6-terra` | 272K / 128K | ChatGPT plan | escalation |
| Codex | `openai-codex` / `gpt-5.6-luna` | 272K / 128K | ChatGPT plan | escalation |
| Codex | `openai-codex` / `gpt-5.5` | 272K / 128K | ChatGPT plan | escalation |
| Codex | `openai-codex` / `gpt-5.3-codex-spark` | 128K / 128K | ChatGPT plan | escalation |

The local engine serves exactly one model (`curl $DS4_BASE_URL/models` →
`DeepSeek-V4-Flash`). The codex seats were **authenticated** at the time of
writing (`pi auth check --provider openai-codex`).

**The 5.6 variants are unbenchmarked on this repo.** luna / terra / sol have not
been compared here, and neither has astra against them. Do not assert one is
better; if the choice matters, run the same brief on two worktrees and measure,
the way the pi-vs-ds4 A/B was run.

## Claude subagents

The `Agent` tool takes a `model` of `opus`, `sonnet`, `haiku` or `fable`. These
cost real money and are **not** for ordinary implementation work.

They are the right seat for exactly two things:

- **Visual and design work** — standing preference: UI tasks split at the
  logic/appearance seam, and appearance goes to Claude. Design quality is the
  local model's weakest axis.
- **Rescue**, after a task has failed two local fix rounds.

A Claude subagent is **invisible to the monitor at :8765**, which globs
`.worktrees/*/.ds4/{transcripts,pi-sessions}/*.jsonl` — Claude seats write none
of those. `scripts/fleet.sh status` labels which seats the dash cannot show and
when each last wrote. Do not synthesise fake pi transcripts to force them onto
the dash: an invented heartbeat is worse than an honest gap.

## Choosing a seat

1. **Default to the local seat.** It is free and good enough when the brief is
   sharp. A vague brief is the usual cause of a bad local diff, not the model.
2. **Visual/design → Claude subagent**, first time, no ladder.
3. **Escalate on evidence, not on the task feeling important.** Two failed local
   fix rounds, or fabricated evidence (a report claiming a test passed that your
   own run fails), or the same failure signature twice.
4. **Codex is the cheaper escalation** — offer it and name a model, then let the
   user pick. It is an alternative to a Claude implementer, not a replacement
   for the two local rounds.
5. **Claude implementer last.**

## Concurrency — two pools, not one cap

User ruling, 2026-09-07. The seat limit is **two independent pools**, because
the two ceilings exist for unrelated reasons:

| pool | cap | claimed with | what the cap protects |
|---|---|---|---|
| **local** | 4 | `fleet.sh claim <id> --local` (the default) | THIS BOX. Past four local agents it saturates and load-sensitive tests fail for reasons unrelated to any diff. |
| **paid** | 2 | `fleet.sh claim <id> --paid` | The PLAN. Codex seats run on someone else's hardware and load the box not at all, but share one ChatGPT plan's rate limit and throttle each other and the user's own sessions. |

So **6 agents can run at once** — 4 local plus 2 paid. A full local pool does
NOT block a codex dispatch, and a full paid pool does not block a local one;
`fleet.sh claim` says which pool is full and points at the other. Counting them
against one number was wrong: it made a free seat and a paid seat
interchangeable when they are limited by different resources entirely.

Pass the pool as a flag. A dispatch knows which seat it is about to spend, and
recovering it afterwards from a transcript is how a paid run gets miscounted as
free. A seat directory with no `kind` file predates pools and counts as local.

Claude subagents load neither pool, so a full fleet routes visual/rescue work
there rather than into a queue.

## Always evaluate an agent's tool errors when it completes

Standing rule, user, 2026-09-07. `STATUS=DONE` does not mean the run was clean.
Every finished agent's tool-error count must be READ and JUDGED before the diff
is gated — the dash shows it per row (`tool errs N`) and the `--out` JSON
carries it as `tool_errors`.

The count on its own means nothing; only the errors themselves do. Pull them
out of the transcript (match each `toolResult` with `isError` back to the
`toolCall` that caused it) and sort each into one of three buckets:

- **Expected** — the error IS the deliverable. A mutation test disables a guard
  and runs the suite; the suite fails; that failure is recorded as a tool error
  even though it proves the guard works. Task mt1 finished with 4 tool errors of
  which 3 were exactly this. Never treat these as a defect.
- **Benign** — a `read` of a path that does not exist, a grep with no match. The
  agent recovered and moved on. Note and drop.
- **Real** — a gate the agent could not run, a command it retried and abandoned,
  a compile failure it worked around rather than fixed. These change the merge
  decision: whatever that command was going to verify is UNVERIFIED, and the
  agent's report may claim otherwise.

Record the bucket counts in the merge note. An agent that reports `DONE` while
its transcript shows a real error on the very gate its report claims to have
run is the "fabricated evidence" case, and it is grounds for escalation.

Related: `--out`'s `report_written` is computed against the WRAPPER's cwd, not
the worktree, so a report written to the brief's worktree-relative path reads as
`false` even when the file is there. Check the file yourself before believing
that field — it is the same shape of lie as `commits: []`.

## Every seat gets a worktree from the script

`scripts/agent-worktree.sh <id> [base] [--web]`. Never `git worktree add` by
hand: it carries only tracked files, and a worktree without the `.cards` symlink
makes every corpus-dependent test **skip** rather than fail — the package still
prints `ok` (in 2ms) and the agent, the pre-filter and the gate all read a green
run that executed nothing. Four worktrees across both threads have been poisoned
this way. The script links the corpus and then *proves* it is reachable before
declaring the worktree ready.

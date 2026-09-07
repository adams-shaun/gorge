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

## Concurrency

- **4 local agents total**, across both orchestrator threads — claimed with
  `scripts/fleet.sh claim`. Past four the box saturates and load-sensitive tests
  fail for reasons unrelated to any diff.
- **Codex seats do not load the box** but share one ChatGPT plan's rate limit,
  and will throttle each other and the user's own sessions. Treat **2
  concurrent** as the ceiling until measured.
- Claude subagents load neither, so a full seat pool routes work there rather
  than into a queue.

## Every seat gets a worktree from the script

`scripts/agent-worktree.sh <id> [base] [--web]`. Never `git worktree add` by
hand: it carries only tracked files, and a worktree without the `.cards` symlink
makes every corpus-dependent test **skip** rather than fail — the package still
prints `ok` (in 2ms) and the agent, the pre-filter and the gate all read a green
run that executed nothing. Four worktrees across both threads have been poisoned
this way. The script links the corpus and then *proves* it is reachable before
declaring the worktree ready.

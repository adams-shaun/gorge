# Queue intake: optional cast payment plans

The implementation authority is
[the specification](../../specs/2026-09-24-cast-payment-plans.md).
The six numbered files are direct implementation briefs, with a sequential
dependency chain and assigned acceptance criteria. They can be queued without
another triage pass. This directory itself is not the live issue ledger.

| File | Issue ID | Depends on |
|---|---|---|
| [01-contract.md](01-contract.md) | payplan-01-contract | none |
| [02-planner.md](02-planner.md) | payplan-02-planner | payplan-01-contract |
| [03-execution.md](03-execution.md) | payplan-03-execution | payplan-02-planner |
| [04-host.md](04-host.md) | payplan-04-host | payplan-03-execution |
| [05-client.md](05-client.md) | payplan-05-client | payplan-04-host |
| [06-acceptance.md](06-acceptance.md) | payplan-06-acceptance | payplan-05-client |

## Intake commands

Run from the gorge checkout, using the installed/pinned agentctl package as the
daemon does. The following commands submit work to the live queue; preparing
this document does not run them. Check for these IDs before intake; existing
IDs must be inspected, not overwritten or silently renamed on one side of a
dependency.

```sh
PYTHONPATH="$HOME/.agentctl/pins/current" python3 -m agentctl issue add . --id payplan-01-contract --title 'Payment plans: protocol and identity contract' --kind payment-plan --priority 2 --brief-file docs/superpowers/plans/cast-payment-plans/01-contract.md
PYTHONPATH="$HOME/.agentctl/pins/current" python3 -m agentctl issue add . --id payplan-02-planner --title 'Payment plans: bounded planner and dormant offers' --kind payment-plan --priority 2 --depends payplan-01-contract --brief-file docs/superpowers/plans/cast-payment-plans/02-planner.md
PYTHONPATH="$HOME/.agentctl/pins/current" python3 -m agentctl issue add . --id payplan-03-execution --title 'Payment plans: cast execution and replay' --kind payment-plan --priority 2 --depends payplan-02-planner --brief-file docs/superpowers/plans/cast-payment-plans/03-execution.md
PYTHONPATH="$HOME/.agentctl/pins/current" python3 -m agentctl issue add . --id payplan-04-host --title 'Payment plans: live host offers and recovery' --kind payment-plan --priority 2 --depends payplan-03-execution --brief-file docs/superpowers/plans/cast-payment-plans/04-host.md
PYTHONPATH="$HOME/.agentctl/pins/current" python3 -m agentctl issue add . --id payplan-05-client --title 'Payment plans: client toggle and per-cast selection' --kind payment-plan --priority 2 --depends payplan-04-host --brief-file docs/superpowers/plans/cast-payment-plans/05-client.md
PYTHONPATH="$HOME/.agentctl/pins/current" python3 -m agentctl issue add . --id payplan-06-acceptance --title 'Payment plans: integrated acceptance and report' --kind payment-plan --priority 2 --depends payplan-05-client --brief-file docs/superpowers/plans/cast-payment-plans/06-acceptance.md
```

The dependency lines in each brief and the CLI --depends values intentionally
agree. agentctl parses Depends-On from report/brief and deduplicates it. Intake
should leave each issue `briefed`; the dependency mechanism holds later slices
until their predecessors are fixed/merged. Omit --triage because the briefs
already define implementation scope and acceptance.

Allow approximately 8-12 engineer-days for the bounded release; this is a scope
estimate rather than a dispatch time limit. Follow-up tickets for broader mana
shapes or multiple plans require separate acceptance and must not grow V1 during
implementation. Completion means the final ticket closes the full PP matrix.

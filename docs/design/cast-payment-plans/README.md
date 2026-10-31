# Implementation briefs: optional cast payment plans

The implementation authority is
[the specification](../2026-09-24-cast-payment-plans.md).
The six numbered files are direct implementation briefs, with a sequential
dependency chain and assigned acceptance criteria. 

| File | Issue ID | Depends on |
|---|---|---|
| [01-contract.md](01-contract.md) | payplan-01-contract | none |
| [02-planner.md](02-planner.md) | payplan-02-planner | payplan-01-contract |
| [03-execution.md](03-execution.md) | payplan-03-execution | payplan-02-planner |
| [04-host.md](04-host.md) | payplan-04-host | payplan-03-execution |
| [05-client.md](05-client.md) | payplan-05-client | payplan-04-host |
| [06-acceptance.md](06-acceptance.md) | payplan-06-acceptance | payplan-05-client |

Allow approximately 8-12 engineer-days for the bounded release; this is a scope
estimate rather than a dispatch time limit. Follow-up tickets for broader mana
shapes or multiple plans require separate acceptance and must not grow V1 during
implementation. Completion means the final ticket closes the full PP matrix.

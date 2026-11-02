# dirac docs

Index for dirac: a second 2-player Magic rules engine built for fast search and
training, held to gorge by lockstep. Start with the front door, then pick the guide that matches the
job.

## The guides

| Doc | Read it when |
|---|---|
| [`dirac/README.md`](../README.md) | You are new and want to drive a game: `State`, `Step`/`Apply`, decisions, actions, `Obj` indices, `Projection`, `Config`, stalls, the state rules, the import boundary and the package map. |
| [`dirac/docs/cards.md`](cards.md) | You need to know how cards get into dirac (`kgen`, binary tables), whether a card is supported, or how to add a card or mechanic. |
| [`dirac/docs/verification.md`](verification.md) | You changed dirac or oracle and need the lockstep, `kfuzz`, parity commands, CR coverage or how to follow a gorge change. |
| [`dirac/docs/running.md`](running.md) | You want to run games or bots: `kfuzz`, the Go API, the SpellBench v2 engine and the benchmarks. |
| [`dirac/docs/serving.md`](serving.md) | You want a server's tables to run on dirac: `gorged -engine dirac`, what it plays and what it refuses. |

## Deep reference

| Doc | Read it when |
|---|---|
| [`dirac/doc.go`](../doc.go) godoc | You need the authoritative API, the projection layout, the decision grammar or the exact gorge rules dirac follows. |
| [`DEVIATIONS.md`](DEVIATIONS.md), [`GORGE-FINDINGS.md`](GORGE-FINDINGS.md) | You need the declared dirac-vs-gorge differences, or the gorge behaviour dirac copies. |
| [`dirac/docs/kgen-params.md`](kgen-params.md) | You are changing `dirac/kgen` lowering and need the (API or trigger/static/replacement mode, parameter) census. |
| [`dirac/crcoverage/README.md`](../crcoverage/README.md) | You need the rule-by-rule CR coverage table and where each rule is implemented. |
| [`dirac/docs/glossary.md`](glossary.md) | You met a term you do not know: lockstep, stall, supported or pool. |
| [`dirac/docs/support.md`](support.md) | You want to know how many cards are supported, refused or unverified, per Standard set. Generated. |

Names: *kop* is dirac's card-effect
vocabulary, *kgen* is the generator, *lockstep* is the gorge-vs-dirac
comparison, and a *stall* is a capacity stop that ends the game as a draw.

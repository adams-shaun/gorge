# restrictvalid

**Stand-in.** `RestrictValid$` is carried into the mana pool; only dotted `Spell.<filter>`/`Activated.<filter>` alternatives supported by the matcher are enforced. Unsupported alternatives fail closed, including bare `Spell`/`Activated` (13 raw lines), `CostContainsX`, `CumulativeUpkeep`, `CantCast*`, `Static.*` and `nonSpell`; 41 raw `RestrictValid$` lines across 39 corpus files contain at least one such term, so mixed alternatives can lose only the unsupported branch.

**Where.** `engine/effects/mana_effect.go` (`effMana`), `engine/rules/pay/payment.go` (`RestrictValidMatches`)

**Removed by.** M4 (the remaining restriction grammar)

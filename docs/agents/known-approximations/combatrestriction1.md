# combatrestriction1

**Stand-in.** (combatrestriction1) Only whitelisted static shapes are enforced; an unwhitelisted parameter means the static is SKIPPED, not enforced blanket — the deliberate permissive direction. `MustAttack$ ChosenPlayer`/`RememberedPlayer` name their defender for real (Territorial Hellkite's ChoosePlayer riders, via `attackRequirementSet`); every OTHER `MustAttack$` player reference stays unread and fails closed, and a NAMED duty whose defender no offered pair reaches releases the creature entirely (CR 508.1d: attacking anyone else obeys zero requirements too), so it is not marked Required. Unread: CantAttack's UnlessDefender$/Condition$/CheckSVar$ family, CantSacrifice's `ForCost$ True`, a `Target$` list's walker half.

**Where.** `engine/rules/layers_restrict.go` (`SacrificeBlocked`), `engine/rules/combat/` (`AttackBlocked`, `AttackRequirements`), `engine/rules/combat.go` (`mustAttackRequired`, `attackDutyDischargeable`), `engine/effects/choose_control.go` (`effChoosePlayer`), `engine/effects/misc.go` (`effEffect`)

**Removed by.** M4 (the conditional parameter grammar; ForCost$ True cost provenance; the walker Target$ half)

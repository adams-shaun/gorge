# effect-modes

**Stand-in.** `Effect` registers a `StaticAbilities$`/`Triggers$` mode for real via `engine/effects/effect.go`'s switch and `trigmatch.Lookup`; these still Note: statics `AssignNoCombatDamage`, `AttackVigilance`, `BlockRestrict`, `CanAdapt`, `CantBecomeMonarch`, `IgnoreShroud`, `MinMaxBlocker`, `UntapOtherPlayer`; triggers `Abandoned`, `Adapt`, `Shuffled`, `VisitAttraction`.

**Where.** `engine/effects/effect.go` (`effEffect`), `engine/effects/effect_ignorehexproof.go`, `engine/rules/trigmatch` (`Lookup`), `engine/rules/layers_restrict.go`, `engine/state/continuous.go`

**Removed by.** M4 (the named modes)

# kw-mayflashsac

**Stand-in.** (kw:MayFlashSac) The keyword is implemented rules-side, not as a `engine/cards/keywords.go` expansion: its meaning is a casting OPTION, which `expandKeywords`' own doc lists as the family rules reads directly, and the sibling `kw:MayFlashCost` landed the same way. The rider ("if you CAST it any time a sorcery couldn't have been cast") rides a pay-time `FlagMayFlashSac` CastInfo, and `state.CastProvenanceFlags` strips that bit from a stack copy, since a copy is put on the stack and never cast (CR 707.10). Deliberately NOT changed with it: the sibling entry-hook bits `FlagEvoked`/`FlagDashed`/`FlagWarped` are still inherited by a copy, because they are conditioned on an alternative COST having been paid — a cast-time choice the copy rules do carry for the comparable kicked case.

**Where.** `engine/rules/mayflashsac.go`, `engine/rules/statics_cast.go` (`spellTimingOK`), `engine/rules/altcast.go` (`altCostEnter`), `engine/state/object.go` (`CastProvenanceFlags`), `engine/events/apply.go` (`StackCopy`)

**Removed by.** M4 (a copy ruling for the evoke/dash/warp entry hooks)

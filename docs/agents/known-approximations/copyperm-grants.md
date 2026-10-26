# copyperm-grants

**Stand-in.** (copyperm-grants) `api:CopyPermanent` now grants named `AddTriggers$`/`AddSVars$`/`AddAbilities$` from the resolving source table, supports a named `AttachedTo$` destination and the exact Zndrsplt `Choices$`/`Chooser$` shape. `WithDifferentNames$` remains loud-unimplemented, while `ImprintTokens$ True` still records no copies for delayed exile/sacrifice (Kharasha Foothills, Shredder, Shadow Master); `TokenRemembered$` is real.

**Where.** `engine/effects/copypermanent.go` (`effCopyPermanent`, unread-parameter Note block)

**Removed by.** M4 (`WithDifferentNames$`, `ImprintTokens$`)

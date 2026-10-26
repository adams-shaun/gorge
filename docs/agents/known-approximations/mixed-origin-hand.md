# mixed-origin-hand

**Stand-in.** An explicit multi-zone `ChangeZone Origin$` including `Hand` has no origin-aware chooser: the exact-hand and exact-library walkers cannot build one private option list across hand, graveyard, library and battlefield, so a source-default picker emits one replay-visible Note and moves nothing loudly.

**Where.** `engine/effects/zone.go` (`mixedOriginIncludesHand`), `engine/effects/zone_change.go` (`effChangeZone`)

**Removed by.** M4 (origin-aware mixed hidden-zone chooser)

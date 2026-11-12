package rules

import "github.com/adams-shaun/gorge/engine/effects"

// CR 702.32 Fading: engine/cards/kw_fading.go's entry counters and the
// upkeep trigger (a sacrifice chained ahead of the removal, so a last counter
// removed leaves the permanent for one more upkeep). The behaviour lives in
// the ordinary replacement and trigger pipelines; only the head needs
// registering (TestFadingBlastoderm*).
func init() { effects.RegisterNonAPI("kw:Fading") }

package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// faceWantsEmerged is the heads-safety gate for the Emerge provenance (the
// faceWantsConvoked shape): only a face that reads Count$Emerged (Adipose
// Offspring) has an emerge cast stamp FlagAltCostPaid and emit the sacrificed
// creature's CastInfo, so every other emerge cast stays byte-identical.
func faceWantsEmerged(f *cards.Face) bool {
	return f != nil && f.Mentions("Count$Emerged")
}

// emergeStamp adds FlagAltCostPaid to an emerge spell cast's flags when its
// face reads Count$Emerged. The CastFlags word is full, so the emerge cast
// rides the alternative-cost bit (on a face printing K:Emerge it names the
// emerge cost, effects' altCostKeywordPaid).
func emergeStamp(pc *pendingCast, face *cards.Face, flags string) string {
	if pc.isAbility() || !pc.emerge || !faceWantsEmerged(face) {
		return flags
	}
	return events.FlagsString(events.FlagsFrom(flags) | state.FlagAltCostPaid)
}

// emergedCastInfo is the trailing pay-time CastInfo (nil for any cast
// emergeStamp did not mark, or whose sacrifice captured nothing) recording
// which creature an Emerge cast sacrificed and its toughness as it last
// existed (CR 608.2h, read from the LKI the sacrifice captured before the
// creature left). flags is the stamped flag string.
func emergedCastInfo(pc *pendingCast, face *cards.Face, flags string, lki []state.SacrificedInfo) []events.Event {
	if pc.isAbility() || !pc.emerge || pc.emergeSac == 0 || !faceWantsEmerged(face) {
		return nil
	}
	for i := range lki {
		if lki[i].Obj == pc.emergeSac {
			return []events.Event{{Kind: events.CastInfo, Obj: pc.card, Amount: lki[i].Toughness,
				Counter: flags, IDs: []state.ObjID{pc.emergeSac}, Text: events.CastInfoEmerged}}
		}
	}
	return nil
}

package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cast-from-graveyard provenance (CR 601.2b, 702.34a): the
// resolving spell's own graveyard-origin cast flag. This is gorge's
// state.ObjectWasCastFromGraveyard (engine/state/object.go), the one read
// both engine/effects/filter.go's wasCastFromGraveyard predicate and
// engine/effects/count_body_heads.go's dotted head call:
//
//	!o.IsCopy && o.CastFlags & (FlagFlashback|FlagHarmonize|FlagJumpstart|FlagEscaped) != 0
//
// gorge adds the zone arm: the flags test OR the
// object's latest cast zone (LatestCastFrom == ZGraveyard), so a spell cast
// from a graveyard through a may-play permission (no flag) reads true too.
// A stack copy reads false (CR 707.10).
//
// The kernel stamps sfFlashback on the spell entry when it is cast for its
// flashback cost (cast.go beginCast; a granted flashback also sets
// sfFbGranted, which the cost builders read), and pushBuild records the zone
// the spell was cast from in stackEntry.prov (bits 0-2) for every table with
// an ExCastFromGrave reader (genPrep.anyCastProv, cast_mayplayoffers.go
// tableCastFromGrave). The copy path clears every flag but
// sfKicked/sfPromised and the prov byte (copy_gen.go mintSpellCopy), and the
// copied stack entry's token carries ofCopy, so a copy reads false through
// every test.

// DebugCastGraveNever makes castFromGrave read false (mutation testing only:
// the lockstep must catch it).
var DebugCastGraveNever bool

// castFromGrave reports whether e is a spell cast with a graveyard-origin
// cast flag or cast from a graveyard. e is the resolving entry: the spell itself when a spell's own
// effect node carries the condition (ConditionDefined$ Self), never a
// triggered ability above it. e == nil or a non-spell entry reads false.
func (s *State) castFromGrave(e *stackEntry) bool {
	return !DebugCastGraveNever && e != nil && e.kind == seSpell &&
		(e.flags&sfFlashback != 0 || kop.Zone(e.prov&7) == kop.ZoneGraveyard) &&
		e.obj != NoObj && s.objs[e.obj].of&ofCopy == 0
}

// gorge's state.ObjectWasCastFromGraveyard (engine/state/object.go)
// is the keyword flags OR the object's latest cast zone: HasLatestCast &&
// LatestCastFrom == ZGraveyard, written by every PutOnStack and cleared by
// any later zone change but the resolution's own stack -> battlefield entry
// (events/apply_zone.go). So a may-play cast from a graveyard (Festival of
// Embers, Osteomancer Adept) reads true, on the stack and, once resolved, on
// the battlefield.

// tableCastFromGrave reports whether some expression of t reads
// ExCastFromGrave: pushBuild then records the cast's zone on the entry
// (stackEntry.prov, genPrep.anyCastProv), which castFromGrave reads.
func tableCastFromGrave(t *kop.Table) bool {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExCastFromGrave {
			return true
		}
	}
	return false
}

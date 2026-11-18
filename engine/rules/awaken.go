package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// castCondCode names the Condition$ values that excuse a chain link's target
// announcement behind a cast option (castSubCondition).
type castCondCode uint8

const (
	castCondOther castCondCode = iota
	castCondKicked
	castCondOptionalCost
	castCondAwakened
)

// castSubCondition is sub's bare Condition$ as a cast-option gate: the three
// values whose outcome is fixed when the cast option is picked, castCondOther
// for everything else (judged as the link resolves).
func castSubCondition(sub *cards.SA) castCondCode {
	word := strings.ToLower(strings.TrimSpace(sub.ParamStr(cards.PKCondition)))
	switch castModeCodes.Code(word) {
	case castModeKicked:
		return castCondKicked
	case castModeOptionalcost:
		return castCondOptionalCost
	}
	if word == altMode(altAwaken) { // the Condition$ word is the cast mode word
		return castCondAwakened
	}
	return castCondOther
}

// awakenLink is the spell ability's "target land you control" link kwAwaken
// minted (Condition$ Awakened, targeted), or nil on a face without it.
func awakenLink(f *cards.Face) *cards.SA {
	if f == nil {
		return nil
	}
	sa := f.SpellAbility()
	for d := 0; sa != nil && d < 64; d++ {
		if castSubCondition(sa) == castCondAwakened && effects.TargetsOf(sa).Targeted() {
			return sa
		}
		sa = sa.Sub
	}
	return nil
}

// awakenLandAvailable reports whether caster p has a legal target for card
// id's awaken link (CR 702.113a: the awakened cast needs a land to target).
func awakenLandAvailable(h awakenTargetReader, g *state.Game, p state.PlayerID, id state.ObjID) bool {
	o := g.Obj(id)
	if o == nil {
		return false
	}
	link := awakenLink(o.Face())
	return link != nil && len(h.legalTargetCandidates(p, id, id, link)) > 0
}

// awakenTargetReader is the target census awakenLandAvailable reads.
type awakenTargetReader interface {
	legalTargetCandidates(p state.PlayerID, source, excludeSelf state.ObjID, sa *cards.SA) []targetCandidate
}

// awakenedCopy reports whether o was cast for its awaken cost (a copy keeps
// the provenance, CR 707.10, and the target its original announced).
func awakenedCopy(o *state.Object) bool {
	if o == nil || o.CastFlags&state.FlagAltCostPaid == 0 {
		return false
	}
	f := o.Face()
	if f == nil {
		return false
	}
	_, ok := f.KeywordParam("Awaken")
	return ok
}

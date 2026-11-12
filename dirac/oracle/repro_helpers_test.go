package oracle

// Shared fixture readers for the repro tests: safe lookups that read a
// recorded game without assuming the object or card is still there.

import (
	"strings"

	dirac "github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// chName returns the face name of g's object id, or "" when the object (or
// its face) is gone.
func chName(g *state.Game, id state.ObjID) string {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// sideObjName is chName for the gorge side of a lockstep pair.
func sideObjName(g *GorgeSide, id state.ObjID) string {
	if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// kopCardNameLower returns the lowercase name of the table's card c, or ""
// when c is out of range.
func kopCardNameLower(tbl *kop.Table, c dirac.CardID) string {
	if int(c) >= len(tbl.Cards) {
		return ""
	}
	return strings.ToLower(tbl.Cards[c].Name)
}

// oneOfEachEntries builds the deck of exactly one copy of each name.
func oneOfEachEntries(names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: n, Count: 1}
	}
	return es
}

// isMoxName reports whether n is one of the five Alpha Moxen.
func isMoxName(n string) bool {
	switch n {
	case "Mox Emerald", "Mox Sapphire", "Mox Jet", "Mox Ruby", "Mox Pearl":
		return true
	}
	return false
}

// reproBattlefieldObj returns g's battlefield object named name controlled
// by ctrl, or nil.
func reproBattlefieldObj(g *state.Game, name string, ctrl state.PlayerID) *state.Object {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Controller != ctrl || o.Card == nil {
			continue
		}
		if f := o.Face(); f != nil && f.Name == name {
			return o
		}
	}
	return nil
}

// reproHandCastAction returns ctrl's cast action for the hand object named
// name, and whether one exists.
func reproHandCastAction(c *PickCtx, g *state.Game, ctrl state.PlayerID, name string) (Canon, bool) {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZHand || o.Owner != ctrl || o.Card == nil {
			continue
		}
		if f := o.Face(); f == nil || f.Name != name {
			continue
		}
		ref := c.G.InstRef(o.ID)
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast && a.Src() == ref }); ok {
			return a, true
		}
	}
	return 0, false
}

// mulliganDonePick is the standard scenario pick with a scenario-specific
// priority answer: keep at the mulligan, done at attackers/blockers, the
// scenario's priority pick at priority, and otherwise the first option.
func mulliganDonePick(prio func(*PickCtx) Canon, c *PickCtx) Canon {
	switch c.D.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DAttackers, DBlockers:
		return pickKind(c, ActDone)
	case DPriority:
		return prio(c)
	}
	return c.D.Options[0]
}

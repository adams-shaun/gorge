package effects

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("Block", effBlock) }

// effBlock is Forge's BlockEffect: each DefinedBlocker$ creature on the
// battlefield blocks each DefinedAttacker$ creature still attacking
// (Balduvian Warlord's "then it blocks an attacking creature of your
// choice", False Orders, Noble Ox's "become blocked by CARDNAME"). It is a
// block made by an effect after (or outside) the declaration, so none of
// the declaration's legality rules apply (CR 509.1); a pair the blocker
// already makes is skipped. The pairs ride one DeclareBlockers event, the
// fold every block takes, with Obj naming the resolving source: that marks
// the event as an effect's, so the "becomes blocked" trigger fires only for
// an attacker that was not already blocked (CR 509.3c), while "blocks" and
// "blocked by a creature" fire per pair as for a declaration.
func effBlock(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	var attackers []state.ObjID
	for _, t := range DefinedSpec(h, c, sa.Params["DefinedAttacker"]) {
		if o := g.Obj(t.Obj); !t.IsPlayer && o != nil && o.Zone == state.ZBattlefield && o.IsAttacking && !slices.Contains(attackers, t.Obj) {
			attackers = append(attackers, t.Obj)
		}
	}
	var blockers []state.ObjID
	for _, t := range DefinedSpec(h, c, sa.Params["DefinedBlocker"]) {
		if o := g.Obj(t.Obj); !t.IsPlayer && o != nil && o.Zone == state.ZBattlefield && h.IsCreature(t.Obj) && !slices.Contains(blockers, t.Obj) {
			blockers = append(blockers, t.Obj)
		}
	}
	if len(attackers) == 0 || len(blockers) == 0 {
		return
	}
	var pairs [][2]state.ObjID
	for _, a := range attackers {
		for _, b := range blockers {
			if !slices.Contains(g.Obj(a).BlockedBy, b) {
				pairs = append(pairs, [2]state.ObjID{a, b})
			}
		}
	}
	if len(pairs) == 0 {
		return
	}
	h.Emit(events.Event{Kind: events.DeclareBlockers, Player: g.Obj(blockers[0]).Controller, Obj: c.Source, Pairs: pairs})
}

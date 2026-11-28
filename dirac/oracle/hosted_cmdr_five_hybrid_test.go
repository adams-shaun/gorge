package oracle

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// fiveHybridProbeScript carries Ulalek, Fused Atrocity's whole printed cost,
// {C/W}{C/U}{C/B}{C/R}{C/G} (CR 702.105): five colourless-or-colour hybrid
// pips, one past gorge's planHybridLimit. gorge's planner declines the shape,
// so gorge casts it through the manual mana window: the caster announces each
// pip's face (pay.AnnouncePip, printed face first) and pays from the pool.
// The kernel carries all five pips (kop.ManaCost.Hyb [5]) and must offer the
// same faces and the same payments.
const fiveHybridProbeScript = "Name:Five Hybrid Probe\nManaCost:CW CU CB CR CG\nTypes:Creature Eldrazi\nPT:2/2\n" +
	"Oracle:Five colourless-or-colour hybrid pips.\n"

// prismRitualProbeScript floats {C}{W}{U}{B}{R}{G} for nothing. gorge's
// planner prices no five-hybrid cast, so the plain cast is offered only when
// the floating pool alone pays it; six mana for five pips leaves every pip a
// real choice of face, which the caster announces and pays from the pool.
const prismRitualProbeScript = "Name:Prism Ritual Probe\nManaCost:0\nTypes:Sorcery\n" +
	"A:SP$ Mana | Produced$ C W U B R G | SpellDescription$ Add {C}{W}{U}{B}{R}{G}.\n" +
	"Oracle:Add {C}{W}{U}{B}{R}{G}.\n"

// fiveHybridProbeDeck is twelve probes and eight rituals over a land base
// with a basic for each face of the pips: Wastes for {C}, Plains, Islands,
// Swamps and Mountains for the colour halves.
func fiveHybridProbeDeck() []string {
	d := append(deckOf(12, "Five Hybrid Probe"), deckOf(8, "Prism Ritual Probe")...)
	for _, l := range []string{"Wastes", "Plains", "Island", "Swamp", "Mountain"} {
		d = append(d, deckOf(4, l)...)
	}
	return d
}

// fiveHybridCastReached: a Five Hybrid Probe entered the battlefield, so its
// five hybrid faces were announced and paid; the case is vacuous otherwise.
func fiveHybridCastReached(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.MoveZone || e.To != state.ZBattlefield {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && !o.IsToken && o.Face() != nil && o.Face().Name == "Five Hybrid Probe" {
			return true
		}
	}
	return false
}

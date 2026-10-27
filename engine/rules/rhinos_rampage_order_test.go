package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 608.2n: the spell is put into its owner's graveyard as the final step of
// its resolution; a creature dealt lethal damage by the fight dies afterwards,
// to state-based actions (CR 704.5g). So the spell's MoveZone to the graveyard
// precedes the fought creature's.
func TestRhinosRampageGraveyardOrder(t *testing.T) {
	rampage := corpusCardText(t, "r/rhinos_rampage.txt")
	const bearSrc = "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

	e, cfg, spell := newFixtureDeckWithOpponentCard(t, 60110, rampage, bearSrc, bearSrc)
	mine := putCreature(t, e, 0, bearSrc)
	opp := putCreature(t, e, 1, bearSrc)
	addMana(t, e, 0, "RG")
	e.Advance()

	opt := castOptionFor(t, e, spell)
	submitChoices(t, e, opt.Index)
	pick := func(want state.ObjID) {
		t.Helper()
		d := e.Pending()
		for d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind[:4] == "pay_" {
			submitChoices(t, e, 0)
			d = e.Pending()
		}
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("expected a target ask, got %+v", d)
		}
		for _, o := range d.Options {
			if o.Obj == want {
				submitChoices(t, e, o.Index)
				return
			}
		}
		t.Fatalf("target %d not offered: %+v", want, d.Options)
	}
	pick(mine)
	pick(opp)
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected ask while draining: %+v", d)
		}
		submitChoices(t, e, 0)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack did not drain: %v", e.G.Stack)
	}

	// Preconditions: the fight happened and both cards are where the rule looks.
	if o := e.G.Obj(opp); o.Zone != state.ZGraveyard {
		t.Fatalf("fought creature zone = %s, want graveyard", o.Zone)
	}
	if o := e.G.Obj(spell); o.Zone != state.ZGraveyard {
		t.Fatalf("spell zone = %s, want graveyard", o.Zone)
	}
	if o := e.G.Obj(mine); o.Zone != state.ZGraveyard {
		t.Fatalf("fighting creature zone = %s, want graveyard (it took 2 from the fight)", o.Zone)
	}
	toGrave := func(id state.ObjID) int {
		for i, ev := range e.L.Events {
			if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZGraveyard {
				return i
			}
		}
		return -1
	}
	si, ci, mi := toGrave(spell), toGrave(opp), toGrave(mine)
	if si < 0 || ci < 0 || mi < 0 {
		t.Fatalf("graveyard MoveZone events: spell=%d fought=%d fighter=%d", si, ci, mi)
	}
	if si > ci || si > mi {
		t.Fatalf("a creature reached the graveyard (fought %d, fighter %d) before the spell (event %d)", ci, mi, si)
	}
	if g := e.G.Zone(state.ZGraveyard, 0); len(g) != 2 || g[0] != spell || g[1] != mine {
		t.Fatalf("seat 0 graveyard = %v, want [spell %d, fighter %d]", g, spell, mine)
	}
	if g := e.G.Zone(state.ZGraveyard, 1); len(g) != 1 || g[0] != opp {
		t.Fatalf("seat 1 graveyard = %v, want [fought %d]", g, opp)
	}
	replayCheck(t, e, cfg)
}

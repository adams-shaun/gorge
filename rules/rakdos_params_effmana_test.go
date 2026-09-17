package rules

// The Rakdos-params brief, gap 10: effMana Amount$ X where X is the
// sacrificed card's mana value (Burnt Offering: "Add X mana in any
// combination of {B} and/or {R}, where X is the sacrificed creature's mana
// value"). The amount resolves through Num's SVar fallback -- SVar:X is
// Sacrificed$CardManaCost, which reads the LKI snapshot of what the cost
// sacrificed (Ctx.Sacrificed) -- so the amount is pinned here on the real
// corpus card. The Combo colour CHOICE is the documented M4 stand-in: the
// executor still adds the amount in EVERY listed colour rather than asking
// for a combination, so the totals below assert the amount half only.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func burntOfferingEngine(t *testing.T, creatureSrc string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Burnt Offering"))
	spells := e.G.Zone(state.ZHand, 0)
	spell := spells[len(spells)-1]
	o := e.G.AddObject(card(t, creatureSrc), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	// The spell's {B} cost pip, plus a float to observe the added mana by.
	addMana(t, e, 0, "BB")
	return e, spell, o.ID
}

func TestBurntOfferingAmountIsTheSacrificedCreatureManaValue(t *testing.T) {
	// A 4-drop creature ({1}{R}{R}{G}): the ability adds X = 4.
	e, spell, creature := burntOfferingEngine(t,
		"Name:Big Boar\nManaCost:1 R R G\nTypes:Creature Boar\nPT:3/3\nOracle:x\n")
	// The SA is the SPELL (SP$ Mana): casting it for {B} plus the sacrifice
	// IS the activation.
	d := e.Pending()
	opt := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == spell {
			opt = o.Index
		}
	}
	if opt < 0 {
		t.Fatalf("Burnt Offering not castable: %+v", d.Options)
	}
	submitChoices(t, e, opt)
	ds := e.Pending()
	if ds == nil || ds.Kind != decision.KChoose || ds.Options[0].Kind != "sacrifice" || ds.Options[0].Obj != creature {
		t.Fatalf("sacrifice ask %+v (creature %d)", ds, creature)
	}
	submitChoices(t, e, ds.Options[0].Index)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(creature).Zone != state.ZGraveyard {
		t.Fatalf("creature zone=%s, want graveyard (the cost)", e.G.Obj(creature).Zone)
	}
	pool := e.G.Players[0].Pool
	// The stand-in shape: X mana in EACH listed colour (the colour-choice ask
	// is the M4 row). X = 3, so 3 black AND 3 red on top of the unspent float.
	if pool[state.MB] != 5 || pool[state.MR] != 4 {
		t.Fatalf("pool=%+v, want 5 black (4 added + float) and 4 red -- X = the sacrificed creature's mana value 4", pool)
	}
}

func TestBurntOfferingAmountTracksAOneDrop(t *testing.T) {
	// A 1-drop creature: X = 1, so 1 of each listed colour.
	e, spell, creature := burntOfferingEngine(t,
		"Name:Small Boar\nManaCost:R\nTypes:Creature Boar\nPT:1/1\nOracle:x\n")
	_ = creature
	d := e.Pending()
	opt := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == spell {
			opt = o.Index
		}
	}
	submitChoices(t, e, opt)
	ds := e.Pending()
	submitChoices(t, e, ds.Options[0].Index)
	passUntilStackEmpty(t, e, 20)
	pool := e.G.Players[0].Pool
	if pool[state.MB] != 2 || pool[state.MR] != 1 {
		t.Fatalf("pool=%+v, want 2 black (1 added + float) and 1 red -- X = the sacrificed creature's mana value 1", pool)
	}
}

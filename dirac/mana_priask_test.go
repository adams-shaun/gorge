package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// priAskScen is a board with one Rhystic-Cave-shaped source (kop.Ability.ManaAskColour |
// ManaUnless, one mana of the chosen colour) for seat 0 and a Mountain for seat 1, at
// seat 0's priority, with Config.PriorityManaAsk as asked.
func priAskScen(t *testing.T, on bool) (sc *scen, cave, mtn Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...) // a table with expressions is a generated one (prepareGen)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExDevotionChosen})
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Unless Cave", Types: kop.TLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColAny, ManaN: 1, ManaAskColour: true, ManaUnless: true}}})
	id := CardID(len(tbl.Cards) - 1)
	sc = newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true, PriorityManaAsk: on})
	cave = sc.perm(id, 0)
	mtn = sc.perm(v.Mountain, 1)
	return sc, cave, mtn
}

// priAskOffered is the activation of obj among the pending priority options.
func priAskOffered(d Decision, obj Obj) (Action, bool) {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(obj) {
			return a, true
		}
	}
	return 0, false
}

// TestManaPriorityActivationAsksAndOpponentPays: under Config.PriorityManaAsk an untapped
// unless source is a priority option. Activating it taps it, asks its controller the colour,
// then each player in seat order whether to pay {1} (the activator, who has nothing to pay
// with, can only decline); the opponent's payment taps its Mountain and the source makes no
// mana. Priority returns to the activator with the pass count cleared (CR 117.4).
func TestManaPriorityActivationAsksAndOpponentPays(t *testing.T) {
	sc, cave, mtn := priAskScen(t, true)
	sc.s.passes = 1 // a pass before the activation: the activation breaks the succession
	d := sc.s.Step()
	act, ok := priAskOffered(d, cave)
	if d.Kind != DkPriority || !ok {
		t.Fatalf("decision %+v: want priority with the Cave's activation offered", d)
	}
	sc.s.Apply(act)
	if !sc.tapped(cave) {
		t.Fatal("the Cave is untapped after its activation: the {T} was not paid before the ask")
	}
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenManaChoice || d.Player != 0 || d.N != 5 {
		t.Fatalf("decision %+v, want the activator's colour ask over WUBRG", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.B)))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPayMode || d.Player != 0 || d.N != 1 {
		t.Fatalf("decision %+v, want the activator's decline-only unless ask", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 0))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPayMode || d.Player != 1 || d.N != 2 {
		t.Fatalf("decision %+v, want the opponent's pay/decline ask", d)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
	d = sc.s.Step()
	if d.Kind != DkPriority || d.Player != 0 {
		t.Fatalf("decision %+v, want seat 0's priority again", d)
	}
	if !sc.tapped(mtn) {
		t.Fatal("the opponent's Mountain is untapped: the {1} was not charged")
	}
	if got := sc.s.pl[0].pool; got != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool %v, want empty: a paid unless adds no mana", got)
	}
	if c := sc.s.perms[sc.slot(cave)].chosen; c != 255 {
		t.Fatalf("the Cave records colour %d, want cleared", c)
	}
	if sc.s.passes != 0 {
		t.Fatalf("passes = %d after the activation, want 0", sc.s.passes)
	}
}

// TestManaPriorityActivationAllDeclineAdds: when every player declines the colour's mana
// floats; and with the knob off the source is no priority option at all.
func TestManaPriorityActivationAllDeclineAdds(t *testing.T) {
	sc, cave, _ := priAskScen(t, true)
	act, ok := priAskOffered(sc.s.Step(), cave)
	if !ok {
		t.Fatal("the Cave's activation is not offered")
	}
	sc.s.Apply(act)
	sc.s.Step()
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.R)))
	for want := uint8(0); want < 2; want++ {
		if d := sc.s.Step(); d.Sub != ChGenPayMode || d.Player != want {
			t.Fatalf("ask %+v, want player %d's unless ask", d, want)
		}
		sc.s.Apply(MakeAction(AkChoose, 0, 0, 0))
	}
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision %+v, want priority", d)
	}
	if got := sc.s.pl[0].pool[kop.R]; got != 1 {
		t.Fatalf("red floating %d, want 1", got)
	}

	off, ocave, _ := priAskScen(t, false)
	if _, ok := priAskOffered(off.s.Step(), ocave); ok {
		t.Fatal("the Cave's activation is offered with Config.PriorityManaAsk off")
	}
}

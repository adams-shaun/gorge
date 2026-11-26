package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestXWindowCondSplitTap: an announced-X cast's CR 601.2g window (xWindowPay)
// walks gorge's manual window, whose answer is the first listed source. A
// Condition-split source (mana_condsplit.go) whose gate holds is planner-
// deferred, and the window takes a deferred source in its battlefield place:
// a lucky Gemstone beside plains is tapped first, its arm's any-colour mana
// floats, and the walk stops when the pool pays.
func TestXWindowCondSplitTap(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gem := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Gemstone", Types: kop.TLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColC, ManaN: 1, SplitGate: kop.SplitGateCounter, SplitCtr: kop.CtrLuck, SplitCols: kop.ColAny}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyCondSplit {
		t.Fatal("precondition: the table has no Condition-split source")
	}
	tapped := []Obj{}
	g := sc.perm(gem, 0)
	sc.s.perms[sc.slot(g)].addCtrN(sc.s, int(kop.CtrLuck), 1)
	for i := 0; i < 6; i++ {
		tapped = append(tapped, sc.perm(v.Plains, 0))
	}
	if !sc.s.planDeferred(sc.k, 0, sc.slot(g)) {
		t.Fatal("precondition: the lucky Gemstone is not planner-deferred")
	}
	if !sc.s.splitGateOn(sc.k, sc.slot(g), &sc.k.t.Cards[gem].Abilities[0]) {
		t.Fatal("precondition: the Gemstone's arm gate does not hold")
	}
	c := kop.ManaCost{Generic: 6}
	// loweredWindowOK's walk for an announced-X lowered cast (cast.go bsPay):
	// clamp = !anyType && !anyColor, noOwed = pipN == 0, takeDeferred = the
	// X announcement, payAsk false.
	ok, held := sc.s.windowTapWalk(sc.k, 0, &c, &c, true, false, true, false, true, false, false, false)
	if !ok {
		t.Fatal("the window walk failed")
	}
	if held != ([6]uint8{}) {
		t.Fatalf("held mana %#v", held)
	}
	if !sc.tapped(g) {
		t.Errorf("the lucky Gemstone was not tapped; tapped plains: %d/6", countTapped(sc, tapped))
	}
	if n := countTapped(sc, tapped); n != 5 {
		t.Errorf("%d plains tapped, want 5 (the gem's one mana covers the sixth)", n)
	}
	var want [kop.NumManaKinds]uint8
	want[kop.W] = 6 // the arm's any-colour answer floats W (windowCol's lowest), five plains make five more
	if got := sc.s.pl[0].pool; got != want {
		t.Errorf("pool %v, want %v", got, want)
	}
}

func countTapped(sc *scen, objs []Obj) int {
	n := 0
	for _, o := range objs {
		if sc.tapped(o) {
			n++
		}
	}
	return n
}

// TestXWindowCondSplitCast drives a full autopay-surface {X}{X} cast beside a
// lucky Gemstone: the seat announces X = 3 and the CR 601.2g window pays
// through gorge's manual window, the first listed source (the Gemstone, first
// in battlefield order) until the pool pays.
func TestXWindowCondSplitCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gem := CardID(len(tbl.Cards))
	ball := gem + 1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Gemstone", Types: kop.TLand,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
				ManaColors: kop.ColC, ManaN: 1, SplitGate: kop.SplitGateCounter, SplitCtr: kop.CtrLuck, SplitCols: kop.ColAny}}},
		kop.CardDef{Name: "Test Ballista", Types: kop.TCreature, Cost: kop.ManaCost{X: 2}, Power: -1, Toughness: -1})
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	if !sc.k.gen.anyCondSplit {
		t.Fatal("precondition: the table has no Condition-split source")
	}
	g := sc.perm(gem, 0)
	sc.s.perms[sc.slot(g)].addCtrN(sc.s, int(kop.CtrLuck), 1)
	var lands []Obj
	for i := 0; i < 6; i++ {
		lands = append(lands, sc.perm(v.Plains, 0))
	}
	sp := sc.hand(ball, 0)
	sc.do(t, AkCast, TCode(sp), scAny)
	answered, colAsked := false, false
	for i := 0; i < 50; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		if d.Kind == DkChoose {
			switch d.Sub {
			case ChX:
				sc.s.Apply(MakeAction(AkChoose, 0, 0, 3)) // X = 3
				answered = true
			case ChGenManaChoice:
				// the lucky Gemstone's arm colour ask, W U B R G over the arm's
				// colours (its ColAny arm lists all five)
				if d.N != 5 {
					t.Fatalf("the arm colour ask lists %d colours, want 5: %v", d.N, d.Options[:d.N])
				}
				sc.s.Apply(MakeAction(AkChoose, 0, 0, 0)) // W
				colAsked = true
			default:
				t.Fatalf("unexpected choose sub %d opts %v", d.Sub, d.Options[:d.N])
			}
			continue
		}
		t.Fatalf("unexpected decision kind %d sub %d opts %v", d.Kind, d.Sub, d.Options[:d.N])
	}
	if !answered {
		t.Fatal("the X ask was never posed")
	}
	if !colAsked {
		t.Fatal("the lucky Gemstone's arm colour ask was never posed")
	}
	if sc.s.ctl != ctlPriority {
		t.Fatalf("the cast never came back to priority (ctl %d)", sc.s.ctl)
	}
	if !sc.tapped(g) {
		t.Errorf("the lucky Gemstone was not tapped; tapped lands %d/6", countTapped(sc, lands))
	}
	if n := countTapped(sc, lands); n != 5 {
		t.Errorf("%d lands tapped, want 5", n)
	}
	// The window floated the six mana and the cast's payment spent it: the
	// pool settles empty at bsPay.
	var want [kop.NumManaKinds]uint8
	if got := sc.s.pl[0].pool; got != want {
		t.Errorf("pool %v, want %v (the cast's payment spent the window's mana)", got, want)
	}
}

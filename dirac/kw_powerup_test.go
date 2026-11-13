package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// powerupScen builds a scenario over the generated power-up pool. Every
// named card is in the matchup; the library is 20 Plains.
func powerupScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	powerupSet := cardsettest.Set(t, "powerup")
	tbl, da, _, err := powerupSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	pl, ok := id["Plains"]
	if !ok {
		t.Fatal("scenario needs Plains")
	}
	return newScenTableLib(t, tbl, pl, Config{NoElide: true}), id
}

// powerupAbility returns card c's first activated ability.
func powerupAbility(t *testing.T, sc *scen, c CardID) *kop.Ability {
	t.Helper()
	for i := range sc.k.t.Cards[c].Abilities {
		if sc.k.t.Cards[c].Abilities[i].Kind == kop.AbActivated {
			return &sc.k.t.Cards[c].Abilities[i]
		}
	}
	t.Fatalf("%s has no activated ability", sc.k.t.Cards[c].Name)
	return nil
}

// powerupActivation reports whether the current priority decision offers o's
// power-up activation.
func powerupActivation(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}

func (sc *scen) nextTurn() {
	sc.s.turn++
	sc.s.newTurnGen()
}

func TestPowerUpEntryTurnReduced(t *testing.T) {
	sc, id := powerupScen(t, "Brave Brawler")
	b := sc.perm(id["Brave Brawler"], 0)
	// {4}{W} reduced by {1}{W} is {3}. The pool pays the reduced cost, so
	// the ability is offered (gorge's planner prices the printed {4}{W} and
	// needs either a witness for it or the pool for the reduced cost).
	sc.s.pl[0].pool[kop.W] = 3
	d := sc.s.Step()
	if !powerupActivation(&d, b) {
		t.Fatalf("entry-turn power-up not offered from three floating white: %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkActivate, TCode(b), 0, 0))
	d = sc.s.Step()
	if d.Kind != DkPriority || powerupActivation(&d, b) {
		t.Fatalf("the power-up was offered twice in one stint (kind %d)", d.Kind)
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	if n := sc.s.perms[sc.slot(b)].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("Brave Brawler has %d +1/+1 counters, want 2", n)
	}
	if got := sc.s.pl[0].pool[kop.W]; got != 0 {
		t.Fatalf("the reduced {3} charge left %d white in the pool", got)
	}

	// A creature that entered on an earlier turn pays the printed {4}{W}:
	// four Plains are not enough, five are.
	sc2, id2 := powerupScen(t, "Brave Brawler")
	b2 := sc2.perm(id2["Brave Brawler"], 0)
	sc2.nextTurn()
	sc2.lands(0, id2["Plains"], 4)
	d = sc2.s.Step()
	if powerupActivation(&d, b2) {
		t.Fatalf("the full {4}{W} power-up was offered with four Plains")
	}
	sc2.lands(0, id2["Plains"], 1)
	d = sc2.s.Step()
	if !powerupActivation(&d, b2) {
		t.Fatalf("the full {4}{W} power-up was not offered with five Plains: %v", d.Options[:d.N])
	}
}

func TestPowerUpHybridUntouched(t *testing.T) {
	// Abomination, Terrifying Titan's shapes: mana cost {3}{R/G}, power-up
	// {5}{R/G}{R/G}. The pure reducer is gorge powerUpReduceCost, which reads
	// only Generic and Colored.
	rg := kop.ColorMask(kop.ColR | kop.ColG)
	got := powerUpReduce(
		kop.ManaCost{Generic: 5, Hyb: [len(kop.ManaCost{}.Hyb)]kop.ColorMask{rg, rg}},
		kop.ManaCost{Generic: 3, Hyb: [len(kop.ManaCost{}.Hyb)]kop.ColorMask{rg}},
	)
	if got.Generic != 2 || got.Pips != (kop.ManaCost{}).Pips || got.Hyb != [len(kop.ManaCost{}.Hyb)]kop.ColorMask{rg, rg} || got.X != 0 {
		t.Fatalf("Abomination entry-turn cost %+v, want {2}{R/G}{R/G} (hybrid untouched)", got)
	}
}

// powerUpPip answers the hybrid pip ask and returns the decision after the
// ability resolves. The pool or the untapped sources must already be set.
func powerUpPip(t *testing.T, sc *scen, o Obj) Decision {
	t.Helper()
	sc.s.Apply(MakeAction(AkActivate, TCode(o), 0, 0))
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPip {
		t.Fatalf("decision after activate %v/%d, want the hybrid pip ask", d.Kind, d.Sub)
	}
	found := false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkChoose && a.Mode() == uint8(kop.R) {
			found = true
		}
	}
	if !found {
		t.Fatalf("the {R/G} pip ask did not offer red: %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.R)))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPip {
		t.Fatalf("second pip decision %v/%d, want the hybrid pip ask", d.Kind, d.Sub)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, uint8(kop.R)))
	return sc.s.Step()
}

// TestPowerUpHybridLive exercises the live hybrid offer, pip ask, charge and
// once gate on Abomination, Terrifying Titan ({3}{R/G} / {5}{R/G}{R/G}).
func TestPowerUpHybridLive(t *testing.T) {
	sc, id := powerupScen(t, "Abomination, Terrifying Titan")
	a := sc.perm(id["Abomination, Terrifying Titan"], 0)
	sc.s.pl[0].pool[kop.R] = 4 // the entry-turn {2}{R/G}{R/G}
	d := sc.s.Step()
	if !powerupActivation(&d, a) {
		t.Fatalf("entry-turn hybrid power-up not offered: %v", d.Options[:d.N])
	}
	d = powerUpPip(t, sc, a)
	if d.Kind != DkPriority {
		t.Fatalf("decision after the hybrid charge %v, want priority", d.Kind)
	}
	if got := sc.s.pl[0].pool[kop.R]; got != 0 {
		t.Fatalf("the entry-turn {2}{R/G}{R/G} left %d red in the pool", got)
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	if n := sc.s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)); n != 1 {
		t.Fatalf("Abomination has %d +1/+1 counters, want 1", n)
	}
	d = sc.s.Step()
	if powerupActivation(&d, a) {
		t.Fatalf("a used hybrid power-up was offered again: %v", d.Options[:d.N])
	}
}

// TestPowerUpHybridSources proves the reduced cost is paid from untapped
// sources when the pool is empty: the planner witnesses the PRINTED {5}{R/G}
// {R/G} (seven Mountains), the charge is the reduced {2}{R/G}{R/G} (four
// Mountains), and powerUpPlanRejected sends it through the manual window.
func TestPowerUpHybridSources(t *testing.T) {
	sc, id := powerupScen(t, "Abomination, Terrifying Titan", "Mountain")
	a := sc.perm(id["Abomination, Terrifying Titan"], 0)
	var lands []Obj
	for i := 0; i < 7; i++ {
		lands = append(lands, sc.perm(id["Mountain"], 0))
	}
	d := sc.s.Step()
	if !powerupActivation(&d, a) {
		t.Fatalf("source-paid hybrid power-up not offered: %v", d.Options[:d.N])
	}
	d = powerUpPip(t, sc, a)
	if d.Kind != DkPriority {
		t.Fatalf("decision after the source-paid hybrid charge %v, want priority", d.Kind)
	}
	tapped := 0
	for _, l := range lands {
		if sc.s.perms[sc.slot(l)].flags&pfTapped != 0 {
			tapped++
		}
	}
	if tapped != 4 {
		t.Fatalf("the reduced {2}{R/G}{R/G} tapped %d Mountains, want 4", tapped)
	}
}

func TestPowerUpColourSpill(t *testing.T) {
	// The reducer's {W} has no matching {W} in the ability's {5}{G}: the
	// excess comes off the generic part (CR 118.7).
	got := powerUpReduce(
		kop.ManaCost{Generic: 5, Pips: [kop.NumManaKinds]uint8{0, 0, 0, 0, 1}},
		kop.ManaCost{Pips: [kop.NumManaKinds]uint8{1}},
	)
	if got.Generic != 4 || got.Pips != [kop.NumManaKinds]uint8{0, 0, 0, 0, 1} {
		t.Fatalf("spilled reduction %+v, want {4}{G}", got)
	}
}

func TestPowerUpOncePerStint(t *testing.T) {
	sc, id := powerupScen(t, "Brave Brawler")
	b := sc.perm(id["Brave Brawler"], 0)
	sc.s.pl[0].pool[kop.W] = 3 // pays the entry-turn {3}
	sc.do(t, AkActivate, uint8(b), 0xff)
	d := sc.s.Step()
	if powerupActivation(&d, b) {
		t.Fatalf("a used power-up was offered again: %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))

	// A flicker makes a new object (CR 400.7): the once-per-stint bit is
	// clear and the entry-turn reduction applies again.
	sc.s.leave(sc.k, sc.slot(b), kop.ZoneExile, false)
	sc.s.enter(sc.k, b, 0, false, kop.ZoneExile)
	sc.s.pl[0].pool[kop.W] = 3
	d = sc.s.Step()
	if !powerupActivation(&d, b) {
		t.Fatalf("the flickered power-up was not offered again: %v", d.Options[:d.N])
	}
}

// TestPowerUpXCost: Stature, Size Shifter's {X}{U}{U} power-up
// is offered when the floating pool pays the CR 702.193b reduced cost at X = 0
// ({X}{U} on her entry turn), and its X is announced against that reduced cost
// and the pool alone (gorge xAsk).
func TestPowerUpXCost(t *testing.T) {
	deck := []cardset.Entry{{Name: "Island", Count: 20}, {Name: "Stature, Size Shifter", Count: 1}}
	kvsSet := cardsettest.Set(t, "vs")
	setup := func(later bool, pool int) (*scen, Obj) {
		tbl, da, _, err := kvsSet.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeUnpriced: true})
		o := sc.perm(id["Stature, Size Shifter"], 0)
		if later {
			sc.nextTurn()
		}
		sc.s.pl[0].pool[kop.U] = uint8(pool)
		return sc, o
	}
	xOptions := func(sc *scen, o Obj) []int {
		d := sc.s.Step()
		if !powerupActivation(&d, o) {
			return nil
		}
		sc.s.Apply(MakeAction(AkActivate, TCode(o), 0, 0))
		d = sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChX {
			t.Fatalf("kind %d sub %d after the activation, want the X ask", d.Kind, d.Sub)
		}
		xs := []int{}
		for i := 0; i < int(d.N); i++ {
			xs = append(xs, int(d.Options[i].Mode()))
		}
		return xs
	}
	eq := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	sc, o := setup(false, 3)
	if xs := xOptions(sc, o); !eq(xs, []int{0, 1, 2}) {
		t.Fatalf("entry turn, pool UUU: X options %v, want 0..2", xs)
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 2))
	for i := 0; i < 12 && sc.s.perms[sc.slot(o)].ctrLow(int(kop.CtrP1P1)) == 0; i++ {
		if d := sc.s.Step(); d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			sc.s.Apply(d.Options[0])
		}
	}
	if n := sc.s.perms[sc.slot(o)].ctrLow(int(kop.CtrP1P1)); n != 2 || sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("X = 2: %d +1/+1 counters, pool %v; want 2 and an empty pool", n, sc.s.pl[0].pool)
	}
	sc, o = setup(false, 1)
	if xs := xOptions(sc, o); !eq(xs, []int{0}) {
		t.Fatalf("entry turn, pool U: X options %v, want {0} (offered on the reduced cost alone)", xs)
	}
	sc, o = setup(true, 3)
	if xs := xOptions(sc, o); !eq(xs, []int{0, 1}) {
		t.Fatalf("later turn, pool UUU: X options %v, want 0..1", xs)
	}
	sc, o = setup(true, 1)
	if xs := xOptions(sc, o); xs != nil {
		t.Fatalf("later turn, pool U: offered with X options %v", xs)
	}
}

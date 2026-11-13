package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ksacScen builds p0's board from the sacmana pool table: each name once
// per listed copy (a token name is the generated token's), untapped and not
// sick, tokens marked as tokens.
func ksacScen(t *testing.T, cfg Config, board ...string) (*scen, map[string]CardID, []Obj) {
	t.Helper()
	set := cardsettest.Set(t, "sacmana")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, cfg)
	var objs []Obj
	for _, n := range board {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the sacmana table", n)
		}
		o := sc.perm(c, 0)
		if tbl.Cards[c].Types&kop.TToken != 0 || n == "c_a_treasure_sac" || n == "u_1_1_fish" {
			sc.s.objs[o].of |= ofToken
		}
		objs = append(objs, o)
	}
	return sc, id, objs
}

func offersAb(s *State, o Obj, ab uint8) bool {
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == o && a.Mode() == ab {
			return true
		}
	}
	return false
}

var ksacCfgs = []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}, {GorgeAutopaySurface: true, NoElide: true}}

// TestSacManaDisjoint: a Treasure sacrificed for Fountainport's {2} is not
// also the token its cost sacrifices (gorge planComposedCost: a sacrifice
// candidate the plan consumes is gone, planLeavesCostPayable), while a Fish
// token, which makes no mana, is a free candidate.
func TestSacManaDisjoint(t *testing.T) {
	const T, F = "c_a_treasure_sac", "u_1_1_fish"
	for _, cfg := range ksacCfgs {
		for _, c := range []struct {
			board []string
			want  bool
		}{
			{[]string{"Swamp", T}, false},
			{[]string{T, T}, false},
			{[]string{"Swamp", T, T}, true},
			{[]string{T, T, T}, true},
			{[]string{"Swamp", "Swamp", T}, true},
			{[]string{"Swamp", "Swamp"}, false},
			{[]string{"Swamp", T, F}, true},
			{[]string{"Basal Thrull", T}, true},
		} {
			sc, _, objs := ksacScen(t, cfg, append([]string{"Fountainport"}, c.board...)...)
			if got := offersAb(sc.s, objs[0], 1); got != c.want {
				t.Errorf("lower %v autopay %v, Fountainport beside %v: draw offered %v, want %v",
					cfg.GorgeLowering, cfg.GorgeAutopaySurface, c.board, got, c.want)
			}
		}
	}
}

// TestSacManaWitnessReserves: with a Swamp, Basal Thrull and the only
// token a Treasure, Fountainport's {2} is paid by the Thrull's {B}{B} and
// the Treasure is the sacrifice (gorge's witness: the plain plan Swamp +
// Treasure consumes the only candidate, so the Treasure is reserved and
// the plan is the Thrull alone).
func TestSacManaWitnessReserves(t *testing.T) {
	for _, cfg := range ksacCfgs {
		sc, _, objs := ksacScen(t, cfg, "Fountainport", "Swamp", "Basal Thrull", "c_a_treasure_sac")
		fp, swamp, thrull, tr := objs[0], objs[1], objs[2], objs[3]
		if !sc.activateAb(fp, 1) {
			t.Fatalf("Fountainport's draw not offered")
		}
		for i := 0; i < 4; i++ {
			d := sc.s.Step()
			if d.Kind == DkPriority {
				break
			}
			sc.s.Apply(d.Options[0])
		}
		if sc.onBF(thrull) || sc.onBF(tr) || !sc.onBF(swamp) || sc.tapped(swamp) {
			t.Errorf("lower %v autopay %v: Thrull on bf %v, Treasure on bf %v, Swamp tapped %v; want the Thrull's mana and the Treasure sacrificed",
				cfg.GorgeLowering, cfg.GorgeAutopaySurface, sc.onBF(thrull), sc.onBF(tr), sc.tapped(swamp))
		}
	}
}

// TestSacShortWithheld: Deadly Dispute with a Swamp and a Treasure, the only
// artifact or creature: on the planned surfaces, paying {1}{B} would consume what the
// additional cost must sacrifice (CR 601.2g-h; gorge planReserving), so it isn't
// offered. A second Swamp pays the mana and leaves the Treasure.
func TestSacShortWithheld(t *testing.T) {
	for _, cfg := range ksacCfgs[1:] {
		sc, id, _ := ksacScen(t, cfg, "Swamp", "c_a_treasure_sac")
		if dd := sc.hand(id["Deadly Dispute"], 0); offersCast(sc.s, dd) {
			t.Errorf("lower %v autopay %v: Deadly Dispute offered with one Swamp", cfg.GorgeLowering, cfg.GorgeAutopaySurface)
		}
		sc, id, _ = ksacScen(t, cfg, "Swamp", "Swamp", "c_a_treasure_sac")
		if dd := sc.hand(id["Deadly Dispute"], 0); !offersCast(sc.s, dd) {
			t.Errorf("lower %v autopay %v: Deadly Dispute not offered with two Swamps", cfg.GorgeLowering, cfg.GorgeAutopaySurface)
		}
	}
}

// TestSacShortReverses: the same board on the full surface, which plans nothing: the
// cast is offered, pays with the Swamp and the Treasure, finds no sacrifice candidate
// and reverses (CR 733.1): the card is back in hand, the Treasure is gone and its mana
// floats.
func TestSacShortReverses(t *testing.T) {
	sc, id, objs := ksacScen(t, ksacCfgs[0], "Swamp", "c_a_treasure_sac")
	dd := sc.hand(id["Deadly Dispute"], 0)
	if !offersCast(sc.s, dd) {
		t.Fatalf("Deadly Dispute not offered")
	}
	sc.do(t, AkCast, TCode(dd), scAny)
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision %d after the cast, want priority (the reversal)", d.Kind)
	}
	pool, _ := sc.s.PoolOf(0)
	n := 0
	for _, u := range pool {
		n += int(u)
	}
	if sc.s.objs[dd].zone != kop.ZoneHand || sc.onBF(objs[1]) || n != 2 {
		t.Errorf("Deadly Dispute zone %d, Treasure on bf %v, pool %d; want hand, gone, 2", sc.s.objs[dd].zone, sc.onBF(objs[1]), n)
	}
}

// TestTapXManaDisjoint: Volrath's Gardens' "{2}, Tap an untapped creature
// you control" is not offered when the only untapped creature is the
// Llanowar Elves its {2} needs (gorge planEachReservation withholds the
// tap candidate from the mana), and is with a second creature.
func TestTapXManaDisjoint(t *testing.T) {
	for _, cfg := range ksacCfgs {
		for _, c := range []struct {
			board []string
			want  bool
		}{
			{[]string{"Forest", "Llanowar Elves"}, false},
			{[]string{"Forest", "Llanowar Elves", "Grizzly Bears"}, true},
			{[]string{"Forest", "Forest", "Llanowar Elves"}, true},
			{[]string{"Llanowar Elves", "Elvish Mystic", "Grizzly Bears"}, true},
			{[]string{"Llanowar Elves", "Elvish Mystic"}, false},
		} {
			sc, _, objs := ksacScen(t, cfg, append([]string{"Volrath's Gardens"}, c.board...)...)
			if got := offersAb(sc.s, objs[0], 0); got != c.want {
				t.Errorf("lower %v autopay %v, Gardens beside %v: offered %v, want %v",
					cfg.GorgeLowering, cfg.GorgeAutopaySurface, c.board, got, c.want)
			}
		}
	}
}

// TestTapXPickOffersWhatTheManaNeeds: once Volrath's Gardens is offered, its
// tap election lists every untapped creature, the Llanowar Elves its {2}
// leans on included (gorge tapPermanentCostAsk offers each candidate; the
// payment is checked after the pick). Electing the Elves leaves the Forest
// alone to pay {2}, so the activation is reversed with nothing tapped (CR
// 733.1) and the Bears pay the cost on the second try.
func TestTapXPickOffersWhatTheManaNeeds(t *testing.T) {
	for _, cfg := range ksacCfgs {
		sc, _, objs := ksacScen(t, cfg, "Volrath's Gardens", "Forest", "Llanowar Elves", "Grizzly Bears")
		gardens, elves, bears := objs[0], objs[2], objs[3]
		if !sc.activateAb(gardens, 0) {
			t.Fatalf("lower %v autopay %v: Gardens not offered", cfg.GorgeLowering, cfg.GorgeAutopaySurface)
		}
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChCostTap {
			t.Fatalf("lower %v autopay %v: decision %d/%d after the activation, want the tap election", cfg.GorgeLowering, cfg.GorgeAutopaySurface, d.Kind, d.Sub)
		}
		var pickElves, pickBears Action
		for i := 0; i < int(d.N); i++ {
			switch Obj(d.Options[i].Source()) {
			case elves:
				pickElves = d.Options[i]
			case bears:
				pickBears = d.Options[i]
			}
		}
		if pickElves == 0 || pickBears == 0 {
			t.Fatalf("lower %v autopay %v: the election offers %d options without both creatures", cfg.GorgeLowering, cfg.GorgeAutopaySurface, d.N)
		}
		sc.s.Apply(pickElves)
		for i := 0; i < 3 && sc.s.Step().Kind != DkPriority; i++ {
			sc.s.Apply(sc.s.Step().Options[0])
		}
		if sc.tapped(elves) || sc.tapped(bears) || sc.tapped(gardens) {
			t.Errorf("lower %v autopay %v: the reversed activation left a cost paid", cfg.GorgeLowering, cfg.GorgeAutopaySurface)
		}
	}
}

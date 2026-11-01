package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tokScen builds p0's board from the tokmana pool table (ksacScen's
// shape): each listed name once, untapped and not sick; a token name
// (the generated token's) is marked as a token.
func tokScen(t *testing.T, cfg Config, board ...string) (*scen, map[string]CardID, []Obj) {
	t.Helper()
	set := cardsettest.Set(t, "tokmana")
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
			t.Fatalf("%s not in the tokmana table", n)
		}
		o := sc.perm(c, 0)
		if tbl.Cards[c].Types&kop.TToken != 0 || n == tokToken {
			sc.s.objs[o].of |= ofToken
		}
		objs = append(objs, o)
	}
	return sc, id, objs
}

const tokToken = "c_1_1_shapeshifter_changeling"

var tokCfgs = []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}, {GorgeAutopaySurface: true, NoElide: true}}

// TestGrantManaTokenOffer: Springleaf Parade's "{T}: Add one mana of any
// color" on a creature token pays Lightning Strike's {R} beside a Forest
// on the plain and lowering surfaces (gorge: a scripted prefix activates
// the token, the printed Forest pays the rest); the autopay surface
// leaves a scripted plain cast out (gorge.go priorityVocabulary); without
// the Parade the token makes no mana.
func TestGrantManaTokenOffer(t *testing.T) {
	for _, cfg := range tokCfgs {
		for _, c := range []struct {
			board []string
			want  bool
		}{
			{[]string{"Springleaf Parade", tokToken, "Forest"}, !cfg.GorgeAutopaySurface},
			{[]string{tokToken, "Forest"}, false},
			{[]string{"Springleaf Parade", "Grizzly Bears", "Forest"}, false}, // not a token
			{[]string{"Cryptolith Rite", "Grizzly Bears", "Forest"}, !cfg.GorgeAutopaySurface},
			{[]string{"Springleaf Parade", tokToken, tokToken}, !cfg.GorgeAutopaySurface}, // both from grants
		} {
			sc, id, _ := tokScen(t, cfg, c.board...)
			ls := sc.hand(id["Lightning Strike"], 0)
			if got := offersCast(sc.s, ls); got != c.want {
				t.Errorf("lower %v autopay %v, board %v: Lightning Strike offered %v, want %v",
					cfg.GorgeLowering, cfg.GorgeAutopaySurface, c.board, got, c.want)
			}
		}
	}
}

// TestGrantManaDual: under Cryptolith Rite a Llanowar Elves has its own
// {G} beside the granted any-colour ability: the planner's view is the
// printed one (manaPCGen, manaGranted false), the offer counts both
// (Shock's {R} is payable), and gorge's AvailableMana omits a source with
// two free abilities (FeatGrantAvail 0) while a token's counts its five
// colours.
func TestGrantManaDual(t *testing.T) {
	sc, id, objs := tokScen(t, Config{NoElide: true}, "Cryptolith Rite", "Llanowar Elves", "Springleaf Parade", tokToken)
	elves, tok := objs[1], objs[3]
	s, k := sc.s, sc.k
	if s.manaGranted(k, sc.slot(elves)) || s.manaPCGen(k, sc.slot(elves)).manaCol != kop.ColG {
		t.Errorf("the Elves' planner view is not its printed {G}")
	}
	if !s.manaGranted(k, sc.slot(tok)) || s.manaPCGen(k, sc.slot(tok)).manaCol != kop.ColAny {
		t.Errorf("the token's planner view is not the granted any colour")
	}
	var gi [maxGrants]int16
	if n := s.grantIdxs(k, sc.slot(tok), &gi); n != 2 {
		t.Errorf("token grants %d, want 2 (Springleaf Parade's and Cryptolith Rite's)", n)
	}
	if w, ok := s.FeatGrantAvail(elves); !ok || w != 0 {
		t.Errorf("Elves FeatGrantAvail %d %v, want 0 true", w, ok)
	}
	if w, ok := s.FeatGrantAvail(tok); !ok || w != 0 {
		t.Errorf("token with two grants FeatGrantAvail %d %v, want 0 true (several free abilities)", w, ok)
	}
	shock := sc.hand(id["Shock"], 0)
	if !offersCast(s, shock) {
		t.Errorf("Shock not offered with the Elves' granted colour")
	}
	// one grant only: the token counts its five colours
	sc2, _, objs2 := tokScen(t, Config{NoElide: true}, "Springleaf Parade", tokToken)
	if w, ok := sc2.s.FeatGrantAvail(objs2[1]); !ok || w != 5 {
		t.Errorf("token FeatGrantAvail %d %v, want 5 true", w, ok)
	}
}

// TestGrantManaSearch: gorge's prefix search (grantSearch) over two
// Springleaf tokens: Counterspell's {U}{U} is found as token U, token U
// (W first, the colours W U B R G in order, each payer state once); a
// cost no prefix pays within potentialScriptNodes is not found (five
// tokens, {G}{G}{G}{G}{G}: the multisets before GGGGG exhaust the
// budget, so gorge leaves the play "unsupported" and never offers it).
func TestGrantManaSearch(t *testing.T) {
	sc, _, _ := tokScen(t, Config{NoElide: true}, "Springleaf Parade", tokToken, tokToken)
	s, k := sc.s, sc.k
	var g grantSrch
	cost := kop.ManaCost{}
	cost.Pips[kop.U] = 2
	if !s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, &g) || g.np != 2 || g.path[0].col != kop.U || g.path[1].col != kop.U {
		t.Fatalf("UU: found %v steps %d cols %d %d", g.np == 2, g.np, g.path[0].col, g.path[1].col)
	}
	// gorge's node count: the root; token 1's colour ask, W; token 2's
	// ask and its five colours (none pays); token 1 U; token 2's ask, W,
	// U: 13 nodes
	if g.nodes != 13 {
		t.Errorf("UU: %d nodes, want 13", g.nodes)
	}
	sc, _, _ = tokScen(t, Config{NoElide: true}, "Springleaf Parade", tokToken, tokToken, tokToken, tokToken, tokToken)
	s, k = sc.s, sc.k
	cost = kop.ManaCost{}
	cost.Pips[kop.G] = 5
	if s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, &g) {
		t.Errorf("GGGGG with five tokens: found within the budget (%d nodes), gorge's search runs out", g.nodes)
	}
	cost = kop.ManaCost{Generic: 3}
	if !s.grantSearch(k, 0, &cost, NoSlot, payDesc{}, &g) || g.np != 3 {
		t.Errorf("{3}: found %v steps %d", g.np == 3, g.np)
	}
}

// TestGrantManaPrefixPays: on the lowering surface the cast taps the
// found prefix into the pool at the announcement and the printed Forest
// pays the rest: after Lightning Strike resolves both are tapped and the
// pool is empty.
func TestGrantManaPrefixPays(t *testing.T) {
	sc, id, objs := tokScen(t, Config{GorgeLowering: true, NoElide: true}, "Springleaf Parade", tokToken, "Forest")
	tok, forest := objs[1], objs[2]
	ls := sc.hand(id["Lightning Strike"], 0)
	sc.do(t, AkCast, uint8(ls), 0)
	for i := 0; i < 6; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if !sc.tapped(tok) || !sc.tapped(forest) {
		t.Errorf("token tapped %v, Forest tapped %v; want both", sc.tapped(tok), sc.tapped(forest))
	}
	if p := sc.s.pl[0].pool; p != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the cast, want empty", p)
	}
}

// TestGrantManaOverflowStalls: a changeling token is a creature token and
// a Sliver, so Springleaf Parade, Cryptolith Rite and fifteen Gemhide
// Slivers grant it seventeen mana abilities, one past maxGrants: the game
// stalls (StallGrants) instead of dropping one; sixteen do not stall.
func TestGrantManaOverflowStalls(t *testing.T) {
	for _, slivers := range []int{maxGrants - 2, maxGrants - 1} {
		board := []string{"Springleaf Parade", "Cryptolith Rite"}
		for i := 0; i < slivers; i++ {
			board = append(board, "Gemhide Sliver")
		}
		board = append(board, tokToken)
		sc, _, objs := tokScen(t, Config{NoElide: true}, board...)
		var gi [maxGrants]int16
		n := sc.s.grantIdxs(sc.k, sc.slot(objs[len(objs)-1]), &gi)
		over := slivers+2 > maxGrants
		if n != maxGrants || sc.s.Stalled() != over || over && sc.s.StallReason() != StallGrants {
			t.Errorf("%d grants: read %d, stall %d (%s), want %d read, stalled %v",
				slivers+2, n, sc.s.StallReason(), StallName(sc.s.StallReason()), maxGrants, over)
		}
	}
}

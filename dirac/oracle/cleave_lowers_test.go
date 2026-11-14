package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCleaveLowers plays two more of the Cleave cards kgen lowers beside
// Fierce Retribution (cleave_lockstep_test.go) in lockstep, each cast for its
// cleave cost with the bracket-free text observed on a resolution the printed
// half cannot produce:
//
//   - Winged Portent ("Draw a card for each creature [with flying] you
//     control", cleave {4}{G}{U}): the decks hold no flyer, so the printed
//     spell draws nothing while the cleaved one draws a card per creature.
//   - Dig Up ("Search your library for a [basic land] card, [reveal it,] put it
//     into your hand", cleave {1}{B}{B}{G}): the printed spell can only fetch
//     a basic land, the cleaved one any card.
//
// gorge prices no cleaved cast in its planner, so (as in TestCleaveLockstep)
// the option exists only while the floating pool pays the cleave cost:
// Channel the Suns and then Dark Ritual float seven mana in the caster's main
// phase, and the policy casts the cleaved spell from it.

// cleaveLowersWitness observes one seed: the cleaved cast of card resolves,
// and what its resolution put into the caster's hand.
type cleaveLowersWitness struct {
	card     string
	cleaved  map[state.ObjID]bool
	evAt     int
	armed    bool
	armedObj state.ObjID
	resolved bool
	toHand   int  // cards moved library -> hand during the cleaved resolution
	draws    int  // draw events during the cleaved resolution
	nonland  bool // one of them was not a land card
}

func (w *cleaveLowersWitness) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if w.evAt > len(evs) {
		w.evAt, w.armed = 0, false
	}
	for i := w.evAt; i < len(evs); i++ {
		e := &evs[i]
		if !w.armed {
			if e.Kind == events.CastInfo && strings.Contains(e.Counter, "cleaved") {
				w.cleaved[e.Obj] = true
			}
			if e.Kind == events.Resolve && e.Text == w.card && w.cleaved[e.Obj] {
				w.armed, w.armedObj, w.resolved = true, e.Obj, true
			}
			continue
		}
		if e.Kind == events.Draw {
			w.draws++
		}
		if e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZHand {
			w.toHand++
			if f := ref.E.G.Obj(e.Obj).Face(); f != nil && !f.IsLand() {
				w.nonland = true
			}
		}
		if e.Kind == events.MoveZone && e.Obj == w.armedObj && e.From == state.ZStack {
			w.armed = false // the spell left the stack: its resolution is over
		}
	}
	w.evAt = len(evs)
	return nil
}

// cleaveLowersPolicy is cleavePolicy for any case card.
type cleaveLowersPolicy struct {
	cleavePolicy
	card      string
	needBears bool // cast a Grizzly Bears before the cleaved spell (it counts creatures)
}

func (p *cleaveLowersPolicy) Pick(c *PickCtx) Canon {
	if c.D.Kind == DPriority && c.G != nil {
		if p.needBears && p.bears < 1 {
			if a, ok := findAction(c, func(a Canon) bool {
				return a.Kind() == ActCast && c.G.M.InstanceCard(int(a.Src())) == cleaveBears
			}); ok {
				p.bears++
				return a
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && a.Mode()&0xf == castModeOther && c.G.M.InstanceCard(int(a.Src())) == p.card {
				p.fired++
				return a
			}
		}
	}
	// the shared policy would match Fierce Retribution; none is in these decks
	return p.cleavePolicy.Pick(c)
}

func TestCleaveLowers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		card      string
		needBears bool
		ok        func(w *cleaveLowersWitness) bool
	}{
		// no flyer on either side: only the bracket-free text draws
		{"Winged Portent", true, func(w *cleaveLowersWitness) bool { return w.draws > 0 }},
		// a nonland card in hand: the printed text fetches a basic land only
		{"Dig Up", false, func(w *cleaveLowersWitness) bool { return w.toHand == 1 && w.nonland }},
	}
	reg := synthReg(t)
	for _, tc := range cases {
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			order := []string{tc.card, cleaveChannel, cleaveRitual, cleaveBears}
			main := cleaveDeck(t, reg, "Forest", map[string]int{tc.card: 4, cleaveChannel: 4, cleaveRitual: 4, cleaveBears: 4}, order)
			opp := cleaveDeck(t, reg, "Plains", map[string]int{cleaveBears: 8, "Hill Giant": 8}, []string{cleaveBears, "Hill Giant"})
			set := printedDelveSet(t, [2][]string{{tc.card, cleaveChannel, cleaveRitual, cleaveBears, "Forest"}, {cleaveBears, "Hill Giant", "Plains"}})
			tbl, _, _, err := set.Matchup(overloadEntries(main), overloadEntries(opp))
			if err != nil {
				t.Fatal(err)
			}
			m, err := NewMatchup(reg, [2]string{"cleave caster", "opponent"}, [2][]*cards.Card{main, opp}, 0)
			if err != nil {
				t.Fatal(err)
			}
			cleavedCasts, resolved := 0, 0
			for seed := uint64(1); seed <= 200; seed++ {
				k, err := SetSide(set, tbl, Config{})(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				pol := &cleaveLowersPolicy{cleavePolicy: cleavePolicy{rnd: RandomPolicy(seed)}, card: tc.card, needBears: tc.needBears}
				w := &cleaveLowersWitness{card: tc.card, cleaved: map[state.ObjID]bool{}}
				res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 8000, MaxTurns: 24, Check: w.check})
				CloseSide(k)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
				}
				cleavedCasts += pol.fired
				if w.resolved {
					resolved++
				}
				if w.resolved && tc.ok(w) {
					t.Logf("seed %d: %s cast cleaved %d time(s); %d draw(s), %d card(s) to hand, nonland %v", seed, tc.card, pol.fired, w.draws, w.toHand, w.nonland)
					return
				}
			}
			t.Fatalf("no seed cast and resolved %s cleaved with the bracket-free text observable (cleaved casts: %d, resolved: %d)", tc.card, cleavedCasts, resolved)
		})
	}
}

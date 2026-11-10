package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Patient Naturalist mills three cards with Imprint$ True, then puts a land
// card from among the imprinted (milled) cards into its controller's hand and
// makes a Treasure when there was none. The lowered Mill writer and the
// IsImprinted pick (kgen TestMillImprintWriterAndFilter, dirac
// TestMillImprint*) are played here in two games both engines play decision by
// decision: TestStdSetReprosLockstep replays the repros, and
// TestImprintMillReprosTakeTheirBranch requires each recorded game to resolve
// the trigger down the branch its name gives, since a lockstep entry alone
// would pass a script that never cast the card.
//
// The games are recorded by TestImprintMillRecord (IMPRINT_MILL_RECORD=1, the
// kernel table built from the decks): seat 0 casts Patient Naturalist first,
// plays lands, takes the first option of every other decision and passes.
type imprintMillCase struct {
	name  string
	decks [2][]string
	land  bool // the milled cards hold a land: it goes to the hand, no Treasure
}

func imprintMillCases() []imprintMillCase {
	return []imprintMillCase{
		{
			name: "imprint_mill_patient_naturalist_land_to_hand", land: true,
			decks: [2][]string{
				append(deckOf(8, "Patient Naturalist"), deckOf(16, "Forest")...),
				deckOf(24, "Plains"),
			},
		},
		{
			name: "imprint_mill_patient_naturalist_no_land_makes_a_treasure",
			decks: [2][]string{
				append(deckOf(14, "Patient Naturalist"), append(deckOf(8, "Forest"), deckOf(2, "Grizzly Bears")...)...),
				deckOf(24, "Plains"),
			},
		},
	}
}

func imprintMillPolicy(m *Matchup) Policy {
	return PolicyFunc(func(p *PickCtx) Canon {
		opts := p.D.Options
		if p.D.Kind == DPriority {
			for _, o := range opts {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Patient Naturalist" {
					return o
				}
			}
			for _, o := range opts {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		return opts[0]
	})
}

// imprintMillOutcome reads the first Patient Naturalist trigger's
// resolution: the cards it imprinted, whether a card went from the
// graveyard to the hand, and whether a token was made, between the first
// imprint and the next Resolve.
func imprintMillOutcome(evs []events.Event, g *state.Game) (imprinted int, toHand, token bool) {
	in := false
	for i := range evs {
		e := &evs[i]
		switch {
		case e.Kind == events.Imprint && e.Text == "clear":
			continue // the Cleanup's ClearImprinted$
		case e.Kind == events.Imprint && !in:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Patient Naturalist" {
				in = true
				imprinted++
			}
		case e.Kind == events.Imprint && in:
			imprinted++
		case in && e.Kind == events.Resolve:
			return
		case in && e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZHand:
			toHand = true
		case in && e.Kind == events.TokenCreate:
			token = true
		}
	}
	return
}

func imprintMillPlay(t *testing.T, reg *cards.Registry, r *Repro, set cardset.Set, pol Policy) (res Result, imprinted int, toHand, token bool) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	rc.Finish = func(g *GorgeSide) { imprinted, toHand, token = imprintMillOutcome(g.E.L.Events, g.E.G) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), imprinted, toHand, token
}

// TestImprintMillReprosTakeTheirBranch replays each recorded game: no
// divergence, every scripted action offered, three cards imprinted, and the
// branch of the case taken.
func TestImprintMillReprosTakeTheirBranch(t *testing.T) {
	reg := synthReg(t)
	for _, c := range imprintMillCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, n, toHand, token := imprintMillPlay(t, reg, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if n != 3 || toHand != c.land || token == c.land {
				t.Fatalf("imprinted %d, graveyard to hand %v, token %v; want 3, %v, %v", n, toHand, token, c.land, !c.land)
			}
		})
	}
}

// TestImprintMillRecord plays the scripted policy over seeds until the
// trigger takes the case's branch and writes the repro. It runs only with
// IMPRINT_MILL_RECORD=1.
func TestImprintMillRecord(t *testing.T) {
	if os.Getenv("IMPRINT_MILL_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range imprintMillCases() {
		set := charmRepeatSet(t, c.decks)
		found := false
		for seed := uint64(1); seed <= 400 && !found; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			res, n, toHand, token := imprintMillPlay(t, reg, r, set, imprintMillPolicy(m))
			if res.Div != nil {
				t.Fatalf("%s seed %d: divergence at %d: %s %s %v", c.name, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if n != 3 || toHand != c.land || token == c.land {
				continue
			}
			r.setActions(res.Actions)
			if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s seed %d: %d decisions", c.name, seed, len(res.Actions))
			found = true
		}
		if !found {
			t.Fatalf("%s: no seed took the branch", c.name)
		}
	}
}

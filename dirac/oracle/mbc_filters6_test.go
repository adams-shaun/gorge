package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCFilters6Lowers plays the two cards the MBC filters-6 family lowers,
// in lockstep against gorge on the mbcfilters6 fixture pool's table (kgen
// refused both cards before):
//
//   - View from Above (Defined$ Parent): its DBBounce sub-ability, DB$
//     ChangeZone Origin$ Stack Destination$ Hand Defined$ Parent, returns the
//     spell on the stack to its owner's hand whenever its controller controls
//     a white permanent (gorge reads "Parent" as definedSpecSelf, the same
//     alias as "Self", effects/context.go);
//   - Molten Birth (Defined$ Parent): after its two Elemental tokens and a
//     won FlipCoin, DBReturn's DB$ ChangeZone returns the spell to hand.
//
// Kuroki, Thief of Talents and Tresserhorn's Lord, Returned carry the
// family's other two spellings (ControlledBy Player.targetedBy, the bare
// "Self" SacValid) but stay refused by other families' blockers (the Play
// node's ConditionCheckSVar, the Spell ValidSA head, the Draw<N> unless-cost
// tokens, the remembered set's lifetime), so no lockstep game can play them;
// their spellings are exercised at the census and the filter parser only.
//
// The expected answer at every decision is gorge's. Without the lowering the
// pool's table refuses both cards and the matchups cannot be built.
var mbcf6Decks = [2]struct {
	deck []cardset.Entry
	card string
	// tokens is Molten Birth's two 1/1 red Elemental tokens entering.
	tokens int
}{
	{
		deck: []cardset.Entry{
			{Name: "View from Above", Count: 6}, {Name: "Savannah Lions", Count: 10}, {Name: "Plains", Count: 12}, {Name: "Island", Count: 12},
		},
		card: "View from Above",
	},
	{
		deck: []cardset.Entry{
			{Name: "Molten Birth", Count: 8}, {Name: "Mountain", Count: 32},
		},
		card:  "Molten Birth",
		tokens: 2,
	},
}

func TestMBCFilters6Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	opp := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Mountain", Count: 24}}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcfilters6")
	if err != nil {
		t.Fatalf("mbcfilters6 tables: %v (run make dirac-tables)", err)
	}
	for _, tc := range mbcf6Decks {
		t.Run(strings.ReplaceAll(tc.card, " ", "_"), func(t *testing.T) {
			decks := [2][]cardset.Entry{tc.deck, opp}
			var cs [2][]*cards.Card
			for s, d := range decks {
				for _, e := range d {
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for range e.Count {
						cs[s] = append(cs[s], c)
					}
				}
			}
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatalf("%s: the pool's table: %v", tc.card, err)
			}
			resolved, returned, tokens := 0, 0, 0
			for seed := uint64(1); seed <= 16; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"filters6-a", "filters6-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				var r mbcf6Reach
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				rc.Finish = func(ref *GorgeSide) { r = mbcf6Scan(ref, tc.card, tc.tokens) }
				res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
				CloseSide(k)
				if err := unmodelledStallError(t.Name(), &res); err != nil {
					t.Errorf("seed %d: %v", seed, err)
				}
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				resolved += r.resolved
				returned += r.returned
				tokens += r.tokens
			}
			t.Logf("%s: %d games; %d resolutions, %d returns to hand, %d tokens", tc.card, 16, resolved, returned, tokens)
			if resolved < 3 {
				t.Errorf("the spell resolved in %d of 16 games, want at least 3", resolved)
			}
			if returned < 3 {
				t.Errorf("the spell returned to its owner's hand in %d of 16 games, want at least 3", returned)
			}
			if tc.tokens > 0 && tokens < 6 {
				t.Errorf("%d tokens entered, want at least 6", tokens)
			}
		})
	}
}

// mbcf6Reach is what the games did with the card, read from gorge's log.
type mbcf6Reach struct {
	resolved int // the spell left the stack (resolved; the return included)
	returned int // the spell returned to its owner's hand off the stack
	tokens   int // Molten Birth's Elementals minted while it resolves
}

func mbcf6Scan(g *GorgeSide, card string, tokens int) mbcf6Reach {
	var r mbcf6Reach
	named := func(id state.ObjID) bool {
		o := g.E.G.Obj(id)
		return o != nil && o.Face() != nil && o.Face().Name == card
	}
	// onStack holds while a spell of the name is on the stack; the return is
	// a stack-to-hand move of the spell itself while it is the resolving one.
	onStack := false
	for _, e := range g.E.L.Events {
		switch {
		case (e.Kind == events.PutOnStack || e.Kind == events.MoveZone && e.To == state.ZStack) && named(e.Obj):
			onStack = true
		case e.Kind == events.MoveZone && e.From == state.ZStack && named(e.Obj):
			r.resolved++
			if e.To == state.ZHand {
				r.returned++
			}
			onStack = false
		case (e.Kind == events.TokenCreate || e.Kind == events.CardToken) && onStack && tokens > 0:
			r.tokens++ // one of Molten Birth's two Elementals
		}
	}
	return r
}

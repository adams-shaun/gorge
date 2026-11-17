package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Zurgo, Thunder's Decree: "During your end step, Warrior tokens you control
// have 'This token can't be sacrificed.'" Mobilize's "sacrifice them at the
// beginning of the next end step" leaves the Warrior tokens of the turn's
// active player on the battlefield; the same sacrifice in the other player's
// end step still happens. The kernel yields the granted CantSacrifice per
// recipient (static_granted.go eachGranted) behind the outer static's
// PlayerTurn + End of Turn gate.
var grantedCantSacDecks = [2][]cardset.Entry{
	{{Name: "Zurgo, Thunder's Decree", Count: 14}, {Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 8}},
	{{Name: "Zurgo, Thunder's Decree", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10}},
}

// runGrantedCantSac plays the matchup over seeds 1..seeds, both surfaces
// alternating, and returns the games and divergences.
func runGrantedCantSac(t *testing.T, seeds int) (games, divs int) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tdm")
	var cs [2][]*cards.Card
	for s, d := range grantedCantSacDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(grantedCantSacDecks[0], grantedCantSacDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"grantedcantsac-a", "grantedcantsac-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !dirac.DebugGrantedCantSacOff {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs
}

func TestGrantedCantSacrificeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs := runGrantedCantSac(t, 200)
	t.Logf("%d games, %d divergent", games, divs)
}

// TestGrantedCantSacrificeMutation proves the matchup reaches the grant: with
// the kernel dropping every granted static (Card.Granted), a Warrior token
// gorge keeps through its controller's end step is sacrificed here.
func TestGrantedCantSacrificeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugGrantedCantSacOff
	dirac.DebugGrantedCantSacOff = true
	defer func() { dirac.DebugGrantedCantSacOff = old }()
	games, divs := runGrantedCantSac(t, 40)
	t.Logf("%d games, %d divergent with the grant dropped", games, divs)
	if divs < games/2 {
		t.Fatalf("only %d of %d games diverged with the grant dropped: the matchup does not reach it", divs, games)
	}
}

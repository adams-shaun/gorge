package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestHBotReusedBoardPicksMatchFreshBoard: the kernel heuristic bot reuses one
// board from decision to decision (hbot's fillAs unsets only the last fill's
// index entries). On games of the committed fdn, blb and edh slices, with
// gorge's per-card facts, a long-lived bot answers every decision as a bot
// built for that decision alone (an untouched board) does: the pick sequence
// is the one the full rebuild made.
func TestHBotReusedBoardPicksMatchFreshBoard(t *testing.T) {
	base, _ := testCorpus(t)
	for _, name := range []string{"fdn", "blb", "edh"} {
		var set cardset.Set
		if name == "fdn" {
			set = fdnSet()
		} else {
			s, err := cardset.ByName(name)
			if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
				t.Skipf("no %s table (make dirac-tables): %v", name, err)
			}
			if err != nil {
				t.Fatal(err)
			}
			set = cardset.AllowUnverified(s)
		}
		dir, remove, err := ExtractSlice(name)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(remove)
		lines, err := ExplicitPairs(filepath.Join(dir, "pairs.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
		games, decisions, withCre := 0, 0, 0
		for _, l := range lines {
			if games == 5 {
				break
			}
			m, tbl, err := SetMatchup(set, c, l.A, l.B, 0)
			if err != nil {
				continue // a pair the pool refuses
			}
			side, err := SetSide(set, tbl, Config{GorgeAutopaySurface: true, PassElision: true})(m, uint64(games)+1)
			if err != nil {
				continue
			}
			games++
			facts := HBotFactsSet(c.Reg, tbl, nil, set)
			bt := hbot.New(facts)
			if games%2 == 0 {
				bt.EnableHoldInstant()
			}
			s := &side.(*realSide).s
			var d dirac.Decision
			for n := 0; n < 2500; n++ {
				s.StepInto(&d)
				if d.Kind == dirac.DkNone {
					break
				}
				i := 0
				if d.N > 1 {
					ref := hbot.New(facts)
					if games%2 == 0 {
						ref.EnableHoldInstant()
					}
					decisions++
					i = bt.Pick(s, &d)
					if j := ref.Pick(s, &d); i != j {
						t.Fatalf("%s game %d (%s v %s) decision %d kind %d: reused bot picks %d, fresh bot %d", name, games, l.A, l.B, n, d.Kind, i, j)
					}
					if d.Kind == dirac.DkAttackers || d.Kind == dirac.DkBlockers {
						withCre++
					}
				}
				s.Apply(d.Options[i])
			}
			CloseSide(side)
		}
		if games < 3 || decisions < 300 || withCre < 5 {
			t.Fatalf("%s: thin coverage: %d games, %d decisions, %d combat declarations", name, games, decisions, withCre)
		}
		t.Logf("%s: %d games, %d decisions (%d combat declarations): reused and fresh bots agree", name, games, decisions, withCre)
	}
}

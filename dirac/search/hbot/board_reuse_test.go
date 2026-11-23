package hbot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// benchDecks reads the committed bench decks of set (dirac/testdata/bench).
func benchDecks(t testing.TB, set string) [][]cardset.Entry {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "bench", set, "*.json"))
	sort.Strings(paths)
	var out [][]cardset.Entry
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Cards []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"cards"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		var es []cardset.Entry
		for _, c := range d.Cards {
			es = append(es, cardset.Entry{Name: c.Name, Count: c.Count})
		}
		out = append(out, es)
	}
	return out
}

// boardDiff names the first census field where a and b differ ("" none): the
// state a fill leaves that a later read can see.
func boardDiff(a, b *board, nobj int) string {
	switch {
	case a.step != b.step || a.isMain != b.isMain || a.firstMain != b.firstMain || a.myTur != b.myTur || a.n != b.n:
		return "step flags"
	case a.life != b.life:
		return "life"
	case a.pool != b.pool || a.poolR != b.poolR:
		return "pool"
	case a.libSize != b.libSize || a.handSize != b.handSize:
		return "library or hand size"
	case a.nStack != b.nStack:
		return "stack length"
	case a.nCre != b.nCre:
		return fmt.Sprintf("nCre %d vs %d", a.nCre, b.nCre)
	case a.nCard != b.nCard:
		return fmt.Sprintf("nCard %d vs %d", a.nCard, b.nCard)
	case a.nbf != b.nbf:
		return "battlefield lengths"
	}
	for i := 0; i < a.nStack; i++ {
		if a.stack[i] != b.stack[i] {
			return fmt.Sprintf("stack entry %d", i)
		}
	}
	for i := 0; i < a.nCre; i++ {
		if a.cre[i] != b.cre[i] {
			return fmt.Sprintf("creature %d: %+v vs %+v", i, a.cre[i], b.cre[i])
		}
	}
	for i := 0; i < a.nCard; i++ {
		if a.card[i] != b.card[i] {
			return fmt.Sprintf("card %d: %+v vs %+v", i, a.card[i], b.card[i])
		}
	}
	for o := 0; o < nobj; o++ {
		if a.creX[o] != b.creX[o] {
			return fmt.Sprintf("creX[%d] %d vs %d", o, a.creX[o], b.creX[o])
		}
		if a.cardX[o] != b.cardX[o] {
			return fmt.Sprintf("cardX[%d] %d vs %d", o, a.cardX[o], b.cardX[o])
		}
	}
	for p := 0; p < int(a.n); p++ {
		for _, o := range a.bf[p][:a.nbf[p]] {
			if a.bfPos[o] != b.bfPos[o] || a.bfCard[o] != b.bfCard[o] {
				return fmt.Sprintf("battlefield object %d", o)
			}
		}
		for i := 0; i < a.nbf[p]; i++ {
			if a.bf[p][i] != b.bf[p][i] {
				return fmt.Sprintf("battlefield seat %d entry %d", p, i)
			}
		}
	}
	return ""
}

// TestBoardReuseMatchesFreshBoard: the bot's board is reused from one
// decision to the next (only the last fill's index entries are unset). Over
// self-play games on the committed fdn and blb bench decks, at every decision
// the reused board, filled in each of the three modes (whole, priority,
// combat), is field for field the board a bot with an untouched, zeroed board
// fills, and both bots answer alike.
func TestBoardReuseMatchesFreshBoard(t *testing.T) {
	decisions, withCre, withCard, shrank := 0, 0, 0, 0
	for _, name := range []string{"fdn", "blb"} {
		decks := benchDecks(t, name)
		if len(decks) < 12 {
			t.Fatalf("%s: %d bench decks", name, len(decks))
		}
		set := cardsettest.Set(t, name)
		for g := 0; g < 6; g++ {
			tbl, da, db, err := set.Matchup(decks[2*g], decks[2*g+1])
			if err != nil {
				t.Fatalf("%s game %d: %v", name, g, err)
			}
			f := NewFacts(tbl)
			bt := New(f)
			if g%2 == 1 {
				bt.EnableHoldInstant()
			}
			s := dirac.NewConfig(tbl, da, db, uint64(g)*0x9e37+3, dirac.Config{})
			var d dirac.Decision
			for n := 0; n < 4000; n++ {
				s.StepInto(&d)
				if d.Kind == dirac.DkNone {
					break
				}
				if d.N <= 1 {
					s.Apply(d.Options[0])
					continue
				}
				me := uint8(n % 2)
				ref := new(Bot)
				*ref = Bot{f: f, hold: bt.hold}
				before := bt.b.nCard
				decisions++
				for k := 0; k < 3; k++ {
					mode := (n + k) % 3
					fill := func(b *board) {
						switch mode {
						case 0:
							b.fill(s, f, me)
						case 1:
							b.fillPriority(s, f, me, &d)
						default:
							b.fillCombat(s, f, me)
						}
					}
					fill(&bt.b)
					fill(&ref.b)
					if why := boardDiff(&bt.b, &ref.b, s.NumObjs()); why != "" {
						t.Fatalf("%s game %d decision %d mode %d seat %d: reused board differs from a fresh one: %s", name, g, n, mode, me, why)
					}
				}
				for p := 0; p < int(bt.b.n); p++ {
					for _, o := range bt.b.bf[p][:bt.b.nbf[p]] {
						if bt.b.bfCard[o] != s.ObjCard(o) {
							t.Fatalf("%s game %d decision %d: bfCard[%d] = %d, ObjCard %d", name, g, n, o, bt.b.bfCard[o], s.ObjCard(o))
						}
					}
				}
				if bt.b.nCre > 0 {
					withCre++
				}
				if bt.b.nCard > 0 {
					withCard++
				}
				if before > bt.b.nCard {
					shrank++
				}
				i, j := bt.Pick(s, &d), ref.Pick(s, &d)
				if i != j {
					t.Fatalf("%s game %d decision %d kind %d: reused bot picks %d, fresh bot %d", name, g, n, d.Kind, i, j)
				}
				s.Apply(d.Options[i])
			}
		}
	}
	if decisions < 1000 || withCre < 300 || withCard < 800 || shrank < 100 {
		t.Fatalf("thin coverage: %d decisions, %d with creatures, %d with cards, %d where the census shrank", decisions, withCre, withCard, shrank)
	}
}

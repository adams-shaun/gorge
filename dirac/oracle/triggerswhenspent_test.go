package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Gilanra, Caller of Wirewood lockstep: a TriggersWhenSpent$ mana source of
// {G} (not Goggles' {R}) whose rider draws a card when its mana pays a spell with
// mana value 6 or greater. gorge's planner defers the source, so only a potential
// play's prefix script or the mana window taps it. Craw Wurm (mana value 6) paid
// with Gilanra's mana fires the rider; Grizzly Bears paid with it, or Craw Wurm
// paid with other mana, does not.
var ktwsDecks = [2][]cardset.Entry{
	{
		{Name: "Gilanra, Caller of Wirewood", Count: 4}, {Name: "Craw Wurm", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Giant Growth", Count: 2}, {Name: "Forest", Count: 20},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 12}, {Name: "Plains", Count: 20},
	},
}

// ktwsTally counts, on gorge's side, the casts by what paid them: a mana value 6 spell
// after Gilanra was tapped (viaBig), a smaller spell after it (viaSmall), a mana value 6
// spell with Gilanra untouched (plainBig), and the riders pushed after a mana value 6 spell
// (ridersBig) or after any other (ridersOther: the rider must never fire there).
type ktwsTally struct{ games, viaBig, viaSmall, plainBig, ridersBig, ridersOther int }

func ktwsRun(t *testing.T, n uint64, tal *ktwsTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gilanra")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktwsDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ktwsDecks[0], ktwsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"tws-a", "tws-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen, gil, big := 0, false, false
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				if ev.Kind == events.Resolve {
					gil = false // a ward or fee payment's tap is no cast's
				}
				o := ref.E.G.Obj(ev.Obj)
				if o == nil || o.Face() == nil {
					continue
				}
				f := o.Face()
				switch ev.Kind {
				case events.Tap:
					if f.Name == "Gilanra, Caller of Wirewood" {
						gil = true
					}
				case events.PutOnStack:
					big = f.Name == "Craw Wurm"
					switch {
					case gil && big:
						tal.viaBig++
					case gil:
						tal.viaSmall++
					case big:
						tal.plainBig++
					}
					gil = false
				case events.GrantTriggerPush:
					switch {
					case f.Name != "Gilanra, Caller of Wirewood":
					case big:
						tal.ridersBig++
					default:
						tal.ridersOther++
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
		CloseSide(k)
		tal.games++
		if res.Stall != "" && res.Stall != "turns" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
}

func TestMBCTriggersWhenSpentLowers(t *testing.T) {
	t.Parallel()
	var tal ktwsTally
	ktwsRun(t, 200, &tal)
	t.Logf("%d games: %+v", tal.games, tal)
	if tal.viaBig < 20 || tal.viaSmall < 10 || tal.plainBig < 20 || tal.ridersBig < 20 {
		t.Errorf("arm coverage too thin: %+v", tal)
	}
	if tal.ridersOther != 0 {
		t.Errorf("gorge pushed %d riders after a spell of mana value under 6: %+v", tal.ridersOther, tal)
	}
}

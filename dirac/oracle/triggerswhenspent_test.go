package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The TriggersWhenSpent$ lockstep beyond Pyromancer's Goggles' {R}: a deferred
// mana source of any one colour whose rider fires when its mana pays a spell the
// rider's ValidCard$ takes. gorge's planner defers the source, so only a
// potential play's prefix script or the mana window taps it. Gilanra, Caller of
// Wirewood ({G}; draw when it pays a spell of mana value 6 or greater): Craw Wurm
// paid with its mana fires the rider; Grizzly Bears paid with it, or Craw Wurm
// paid with other mana, does not. Gilanra is legendary: the kernel models one floating
// deferred source per player, and a second of the same card cannot stay in play.
type ktwsMatchup struct {
	src, big string // the deferred source; the spell its rider fires for
	decks    [2][]cardset.Entry
}

var ktwsMatchups = []ktwsMatchup{
	{"Gilanra, Caller of Wirewood", "Craw Wurm", [2][]cardset.Entry{
		{
			{Name: "Gilanra, Caller of Wirewood", Count: 4}, {Name: "Craw Wurm", Count: 8}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Giant Growth", Count: 2}, {Name: "Forest", Count: 20},
		},
		{{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 12}, {Name: "Plains", Count: 20}},
	}},
}

// ktwsTally counts, on gorge's side, the casts by what paid them: the big spell
// after the source was tapped (viaBig), a smaller spell after it (viaSmall), the
// big spell with the source untouched (plainBig), and the riders pushed after the
// big spell (ridersBig) or after any other (ridersOther: the rider must never fire
// there).
type ktwsTally struct{ games, viaBig, viaSmall, plainBig, ridersBig, ridersOther int }

func ktwsRun(t *testing.T, mu ktwsMatchup, n uint64, tal *ktwsTally) {
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
	for s, d := range mu.decks {
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
			t.Fatalf("%s deck %d: %d cards", mu.src, s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(mu.decks[0], mu.decks[1])
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
		seen, src, big := 0, false, false
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				if ev.Kind == events.Resolve {
					src = false // a ward or fee payment's tap is no cast's
				}
				o := ref.E.G.Obj(ev.Obj)
				if o == nil || o.Face() == nil {
					continue
				}
				f := o.Face()
				switch ev.Kind {
				case events.Tap:
					if f.Name == mu.src {
						src = true
					}
				case events.PutOnStack:
					big = f.Name == mu.big
					switch {
					case src && big:
						tal.viaBig++
					case src:
						tal.viaSmall++
					case big:
						tal.plainBig++
					}
					src = false
				case events.GrantTriggerPush:
					switch {
					case f.Name != mu.src:
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
			t.Errorf("%s seed %d: stalled: %s", mu.src, seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("%s seed %d last actions %v", mu.src, seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu.src, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
}

func TestMBCTriggersWhenSpentLowers(t *testing.T) {
	t.Parallel()
	for _, mu := range ktwsMatchups {
		var tal ktwsTally
		ktwsRun(t, mu, 200, &tal)
		t.Logf("%s: %d games: %+v", mu.src, tal.games, tal)
		// every arm is reached, and gorge's rider fires only for the big spell
		if tal.viaBig < 20 || tal.viaSmall < 10 || tal.plainBig < 20 || tal.ridersBig < 20 {
			t.Errorf("%s: arm coverage too thin: %+v", mu.src, tal)
		}
		if tal.ridersOther != 0 {
			t.Errorf("%s: gorge pushed %d riders after another spell: %+v", mu.src, tal.ridersOther, tal)
		}
	}
}

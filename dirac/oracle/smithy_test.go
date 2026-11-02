package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Thousand Moons Smithy lockstep: the front face makes a
// Gnome Soldier token and transforms by tapping five artifacts and/or
// creatures at the first main phase; the back face, Barracks of the
// Thousand, taps for {W} and makes a Gnome Soldier whenever its controller
// casts an artifact or creature spell using mana it produced (gorge
// ValidSA$ Spell.ManaFromCard.StrictlySelf: Object.ManaSpentSources). The
// expected answer at every decision is gorge's.
var ksmithyDecks = [][2][]cardset.Entry{{
	{
		{Name: "Thousand Moons Smithy", Count: 8}, {Name: "Ornithopter", Count: 8}, {Name: "Savannah Lions", Count: 8},
		{Name: "Shock", Count: 2}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Thousand Moons Smithy", Count: 8}, {Name: "Ornithopter", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Wind Drake", Count: 4}, {Name: "Shock", Count: 2}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4},
	},
}}

// ksmithyTally counts, on gorge's side, Barracks' triggers and the
// transforms, and the casts Barracks' mana paid for that its trigger ignored.
type ksmithyTally struct {
	trig, flips, games int
}

func ksmithyRun(t *testing.T, n uint64, report bool, tal *ksmithyTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("smithy")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range ksmithyDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"sm-a", "sm-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; seen < len(evs) && tal != nil; seen++ {
					switch ev := evs[seen]; ev.Kind {
					case events.TriggerPush:
						if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Barracks of the Thousand" {
							tal.trig++
						}
					case events.FlipFace:
						tal.flips++
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

func TestSmithyLockstep(t *testing.T) {
	t.Parallel()
	var tal ksmithyTally
	ksmithyRun(t, 600, true, &tal)
	t.Logf("%d games: Barracks triggers %d, transforms %d", tal.games, tal.trig, tal.flips)
	if tal.trig < 100 || tal.flips < 100 {
		t.Errorf("Barracks triggers %d, transforms %d; want at least 100 each", tal.trig, tal.flips)
	}
}

// TestSmithyLockstepMutation: with the reader never matching, and with it
// matching every cast, some seed diverges.
func TestSmithyLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []uint8{1, 2} {
		dirac.SetDebugSonManaMutate(m)
		n := ksmithyRun(t, 100, false, nil)
		dirac.SetDebugSonManaMutate(0)
		if n == 0 {
			t.Errorf("mutation %d: no seed diverged", m)
		}
	}
}

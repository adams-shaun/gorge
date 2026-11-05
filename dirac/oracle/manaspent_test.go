package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The mana-spent riders lockstep: Carnelian Orb of Dragonkind's {R} gives a
// Dragon creature spell it paid for haste until end of turn (AddsKeywords$),
// a changeling creature spell included (Barkform Harvester, CR 702.73a), and
// does nothing for any other spell (a Hill Giant, a Grizzly Bears, a Shock, a
// Giant Growth). A Dragon cast face down (CR 708.4) is pinned in the kernel
// (TestManaHasteRiderMatchesTheSpell): a face-down cast pays from floating
// mana only, so the Orb's tap never pays one here. The expected answer at every decision is gorge's,
// on both surfaces. Pyromancer's Goggles (TriggersWhenSpent$) stays refused:
// gorge's planner defers its source (kgen lower_manaspent.go).
var kmanaspentDecks = [][2][]cardset.Entry{{
	{
		{Name: "Carnelian Orb of Dragonkind", Count: 6}, {Name: "Dragon Hatchling", Count: 6}, {Name: "Barkform Harvester", Count: 4},
		{Name: "Imperial Hellkite", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Flame Slash", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 16},
	},
	{
		{Name: "Carnelian Orb of Dragonkind", Count: 5}, {Name: "Grizzly Bears", Count: 4}, {Name: "Giant Growth", Count: 3},
		{Name: "Lava Axe", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Dragon Hatchling", Count: 3}, {Name: "Barkform Harvester", Count: 2},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 6},
	},
}}

// kmanaspentTally counts, on gorge's side, the casts Carnelian's tap paid
// for (by whether the rider applies) and the grants.
type kmanaspentTally struct {
	orbDragon, orbOther int // Carnelian tapped while paying a Dragon / another spell
	orbChangeling       int // ... a changeling creature spell
	haste, hasteChg     int // AddsKeywords grants, those to the changeling
	games               int
}

func kmanaspentRun(t *testing.T, n uint64, report bool, tal *kmanaspentTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manaspent")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kmanaspentDecks {
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
			m, err := NewMatchup(reg, [2]string{"ms-a", "ms-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen, cur := 0, ""
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; seen < len(evs) && tal != nil; seen++ {
					ev := evs[seen]
					name := ""
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
						name = o.Face().Name
					}
					switch ev.Kind {
					case events.PutOnStack:
						cur = name
					case events.CastInfo:
						if events.FlagsFrom(ev.Counter)&state.FlagAddsKeywords != 0 {
							tal.haste++
							if cur == "Barkform Harvester" {
								tal.hasteChg++
							}
						}
						cur = ""
					case events.Tap:
						switch {
						case cur == "":
						case name == "Carnelian Orb of Dragonkind" && cur == "Dragon Hatchling":
							tal.orbDragon++
						case name == "Carnelian Orb of Dragonkind" && cur == "Barkform Harvester":
							tal.orbChangeling++
						case name == "Carnelian Orb of Dragonkind":
							tal.orbOther++
						}
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

func TestManaSpentRidersLockstep(t *testing.T) {
	t.Parallel()
	var tal kmanaspentTally
	kmanaspentRun(t, 200, true, &tal)
	t.Logf("%d games: %+v", tal.games, tal)
	// both arms are reached: the rider applies (haste) and does not (a
	// non-Dragon spell paid with the rider's mana)
	if tal.orbDragon < 10 || tal.orbOther < 10 || tal.haste < 10 {
		t.Errorf("rider coverage too thin: %+v", tal)
	}
	// the changeling is a Dragon creature spell on gorge's side (a grant),
	// so the lockstep compared dirac on it
	if tal.orbChangeling < 3 || tal.hasteChg < 3 {
		t.Errorf("changeling coverage: %+v", tal)
	}
}

// TestManaSpentRidersLockstepMutation: with Carnelian's haste dropped, some
// seed diverges.
func TestManaSpentRidersLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugManaHasteOff = true
	n := kmanaspentRun(t, 100, false, nil)
	dirac.DebugManaHasteOff = false
	if n == 0 {
		t.Errorf("haste dropped: no seed diverged")
	}
}

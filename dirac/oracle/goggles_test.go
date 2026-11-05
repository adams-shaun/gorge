package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Pyromancer's Goggles lockstep: Goggles' {R} pays a red instant or
// sorcery and the spell is copied (new targets offered); a green spell or a
// red spell paid with other mana is not. gorge's planner defers the source
// (TriggersWhenSpent$), so only a potential play's prefix script or the mana
// window taps it. The expected answer at every decision is gorge's, on both
// surfaces. Monastery Swiftspear's prowess co-fires with the rider on one
// cast: gorge queues the rider after the ordinary cast triggers. Both sides
// pose the trigger-order options sorted, so the queue position itself is
// pinned by dirac's TestDeferredRiderQueuesAfterCastTriggers; here the
// co-fire games must stay in lockstep.
var kgogglesDecks = [][2][]cardset.Entry{{
	{
		{Name: "Pyromancer's Goggles", Count: 8}, {Name: "Flame Slash", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Lava Axe", Count: 4}, {Name: "Monastery Swiftspear", Count: 4}, {Name: "Mountain", Count: 16},
	},
	{
		{Name: "Pyromancer's Goggles", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Giant Growth", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	// targeting Armored Armadillo asks its ward {1} of the Goggles player,
	// paid from the mana window, where gorge activates a deferred source too
	// (windowTaps)
	{
		{Name: "Pyromancer's Goggles", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Flame Slash", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 16},
	},
	{
		{Name: "Armored Armadillo", Count: 12}, {Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 8},
	},
}}

// kgogglesTally counts, on gorge's side, the casts by what paid them: a red
// instant or sorcery paid with Goggles' mana (viaRed: the rider must fire),
// a spell that is not one paid with it (viaOther: it must not), a red
// instant or sorcery paid with other mana (plainRed: it must not), and the
// riders pushed; cofire, a viaRed cast whose caster controls a Monastery
// Swiftspear (prowess and the rider trigger together), cofireAfter those where
// the Swiftspear follows the tapped Goggles in battlefield order (a listener
// walk would list the rider first, gorge lists it last); windowTaps, a Goggles
// tap no cast spent before the next resolution (a ward payment's).
type kgogglesTally struct {
	games, viaRed, viaOther, plainRed, riders, cofire, cofireAfter, windowTaps int
}

func kgogglesRun(t *testing.T, n uint64, report bool, tal *kgogglesTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("goggles")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kgogglesDecks {
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
			m, err := NewMatchup(reg, [2]string{"gg-a", "gg-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen, goggles, tapped := 0, false, state.ObjID(0)
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; seen < len(evs) && tal != nil; seen++ {
					ev := evs[seen]
					if ev.Kind == events.Resolve {
						if goggles {
							tal.windowTaps++
						}
						goggles = false // a ward payment's tap is no cast's
					}
					o := ref.E.G.Obj(ev.Obj)
					if o == nil || o.Face() == nil {
						continue
					}
					f := o.Face()
					switch ev.Kind {
					case events.Tap:
						if f.Name == "Pyromancer's Goggles" {
							goggles, tapped = true, ev.Obj
						}
					case events.PutOnStack:
						redIS := (f.IsInstant() || f.IsSorcery()) && strings.Contains(f.ManaCost, "R")
						switch {
						case goggles && redIS:
							tal.viaRed++
							before := false // the tapped Goggles precedes a Swiftspear in battlefield order
							for _, id := range ref.E.G.Zone(state.ZBattlefield, o.Controller) {
								before = before || id == tapped
								if b := ref.E.G.Obj(id); b != nil && b.Face() != nil && b.Face().Name == "Monastery Swiftspear" {
									tal.cofire++
									if before {
										tal.cofireAfter++
									}
									break
								}
							}
						case goggles:
							tal.viaOther++
						case redIS:
							tal.plainRed++
						}
						goggles = false
					case events.GrantTriggerPush:
						if f.Name == "Pyromancer's Goggles" {
							tal.riders++
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

func TestGogglesDeferredManaCopy(t *testing.T) {
	t.Parallel()
	var tal kgogglesTally
	if n := kgogglesRun(t, 300, true, &tal); n != 0 {
		t.Fatalf("%d seeds diverged", n)
	}
	t.Logf("%d games: %+v", tal.games, tal)
	// every arm is reached, and gorge's rider fires exactly for the first
	if tal.viaRed < 10 || tal.viaOther < 3 || tal.plainRed < 10 || tal.cofireAfter < 1 || tal.windowTaps < 1 {
		t.Errorf("arm coverage too thin: %+v", tal)
	}
	if tal.riders != tal.viaRed {
		t.Errorf("gorge pushed %d riders for %d red instants or sorceries Goggles paid for", tal.riders, tal.viaRed)
	}
}

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The per-card ChangesZone zone-pair lockstep: Along the
// Crooked Way and Kishla Skimmer (a card leaves your graveyard), Justice
// (another nonland permanent returned to hand), Syr Vondam and Three Tree
// Scribe (a battlefield leave to a destination set) beside Raise Dead, Regrowth, Unsummon,
// Cloudshift, Journey to Nowhere, Swords to Plowshares, Murder and Shock. The
// gorge log is tallied: a TriggerPush on a carrier after the move that
// decides it.

var kczpairsDecks = [2][]cardset.Entry{
	{
		{Name: "Justice, Vance Astrovik", Count: 4}, {Name: "Three Tree Scribe", Count: 3},
		{Name: "Kishla Skimmer", Count: 3}, {Name: "Along the Crooked Way", Count: 3},
		{Name: "Unsummon", Count: 3}, {Name: "Regrowth", Count: 2}, {Name: "Raise Dead", Count: 2},
		{Name: "Murder", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Syr Vondam, Sunstar Exemplar", Count: 4},
		{Name: "Journey to Nowhere", Count: 4}, {Name: "Cloudshift", Count: 4},
		{Name: "Swords to Plowshares", Count: 2}, {Name: "Raise Dead", Count: 3},
		{Name: "Murder", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Hill Giant", Count: 3},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 4},
	},
}

// kczpairsDecks2 is the second matchup: the other cards the per-card pairs
// lower (Imperial Cosmographer, Tameshi, Stormfront Riders, Oasis of Renewal,
// Psychomancer, Shard of the Void Dragon, Slagstone Refinery) with the
// artifact and creature support that moves them.
var kczpairsDecks2 = [2][]cardset.Entry{
	{
		{Name: "Imperial Cosmographer", Count: 3}, {Name: "Tameshi, Reality Architect", Count: 3},
		{Name: "Stormfront Riders", Count: 3}, {Name: "Oasis of Renewal", Count: 3},
		{Name: "Unsummon", Count: 2}, {Name: "Boomerang", Count: 3}, {Name: "Cloudshift", Count: 2},
		{Name: "Regrowth", Count: 2}, {Name: "Murder", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 2}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Psychomancer", Count: 3}, {Name: "Shard of the Void Dragon", Count: 4},
		{Name: "Slagstone Refinery", Count: 3}, {Name: "Mind Stone", Count: 3}, {Name: "Sol Ring", Count: 3},
		{Name: "Shatter", Count: 3}, {Name: "Swords to Plowshares", Count: 1}, {Name: "Journey to Nowhere", Count: 2},
		{Name: "Hill Giant", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Plains", Count: 3}, {Name: "Mountain", Count: 4},
	},
}

// kczpairsTally counts carrier pushes after the move that decides them.
type kczpairsTally struct {
	grave, justice, syrScribe int
	deathBeside               int // a creature dying beside a Justice or Three Tree Scribe on the battlefield
	bounceBesideSyr           int // a battlefield-to-hand move with Syr Vondam on the battlefield (his set is graveyard and exile: it stays silent)
	scribeExileLib            int // Three Tree Scribe fires on a battlefield-to-exile or -library move
	other                     int // the second matchup's carriers fire on the move that decides them
	byCard                    map[string]int
}

// kczpairsPolicy casts or activates whenever it can, else random, and scans
// the gorge log as it goes.
func kczpairsPolicy(seed uint64, tl *kczpairsTally) Policy {
	rnd := RandomPolicy(seed)
	at := 0
	var win []events.Event // the zone moves since the resolution began; the pushes follow it
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			evs := c.G.E.L.Events
			if len(evs) < at {
				at = 0
			}
			beside := func(names ...string) bool {
				for p := state.PlayerID(0); p < 2; p++ {
					for _, id := range g.Zone(state.ZBattlefield, p) {
						if o := g.Obj(id); o != nil && o.Face() != nil {
							for _, n := range names {
								if o.Face().Name == n {
									return true
								}
							}
						}
					}
				}
				return false
			}
			for j := at; j < len(evs); j++ {
				e := &evs[j]
				switch e.Kind {
				case events.MoveZone:
					win = append(win, *e)
					if e.From == state.ZBattlefield && e.To == state.ZGraveyard && beside("Justice, Vance Astrovik", "Three Tree Scribe") {
						tl.deathBeside++
					}
					if e.From == state.ZBattlefield && e.To == state.ZHand && beside("Syr Vondam, Sunstar Exemplar") {
						tl.bounceBesideSyr++
					}
				case events.StepChange, events.TurnChange, events.PutOnStack, events.Resolve:
					win = win[:0]
				case events.TriggerPush:
					o := g.Obj(e.Obj)
					if o == nil || o.Face() == nil {
						continue
					}
					moved := func(from, to state.Zone) bool {
						for _, m := range win {
							if m.From == from && (to == 0 || m.To == to) && m.Obj != e.Obj {
								return true
							}
						}
						return false
					}
					switch name := o.Face().Name; {
					case name == "Along the Crooked Way" || name == "Kishla Skimmer":
						if moved(state.ZGraveyard, 0) {
							tl.grave++
						}
					case name == "Justice, Vance Astrovik":
						if moved(state.ZBattlefield, state.ZHand) {
							tl.justice++
						}
					case name == "Syr Vondam, Sunstar Exemplar" || name == "Three Tree Scribe":
						if moved(state.ZBattlefield, 0) {
							tl.syrScribe++
						}
						if name == "Three Tree Scribe" && (moved(state.ZBattlefield, state.ZExile) || moved(state.ZBattlefield, state.ZLibrary)) {
							tl.scribeExileLib++
						}
					case name == "Imperial Cosmographer" || name == "Tameshi, Reality Architect" || name == "Stormfront Riders" ||
						name == "Psychomancer" || name == "Shard of the Void Dragon" || name == "Slagstone Refinery":
						if moved(state.ZBattlefield, 0) {
							tl.other++
							tl.byCard[name]++
						}
					case name == "Oasis of Renewal":
						if moved(state.ZGraveyard, 0) {
							tl.other++
							tl.byCard[name]++
						}
					}
				}
			}
			at = len(evs)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || seed%4 == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func runCzpairs(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl kczpairsTally) {
	t.Helper()
	tl.byCard = map[string]int{}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("czpairs")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCzPairsNoDest
	dirac.DebugCzPairsNoDest = mutate
	defer func() { dirac.DebugCzPairsNoDest = old }()
	for _, pair := range [...]*[2][]cardset.Entry{&kczpairsDecks, &kczpairsDecks2} {
		g, d, st := runCzpairsPair(t, reg, set, pair, seeds, mutate, &tl)
		games, divs, stalls = games+g, divs+d, stalls+st
	}
	return games, divs, stalls, tl
}

func runCzpairsPair(t *testing.T, reg *cards.Registry, set cardset.Set, pair *[2][]cardset.Entry, seeds int, mutate bool, tlp *kczpairsTally) (games, divs, stalls int) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range pair {
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
	tbl, _, _, err := set.Matchup(pair[0], pair[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kczpairs-a", "kczpairs-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kczpairsPolicy(seed, tlp), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				lo := d.At - 8
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
	}
	return games, divs, stalls
}

func TestCzpairsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runCzpairs(t, 256, false)
	t.Logf("second-matchup fires by card: %v", tl.byCard)
	for _, n := range []string{"Imperial Cosmographer", "Tameshi, Reality Architect", "Stormfront Riders", "Oasis of Renewal", "Psychomancer", "Shard of the Void Dragon", "Slagstone Refinery"} {
		if tl.byCard[n] == 0 {
			t.Errorf("%s never fired", n)
		}
	}
	t.Logf("%d games, %d divergences, %d stalls, graveyard-leave fires %d, Justice bounce counters %d, Syr Vondam / Scribe leaves %d, deaths beside Justice or Scribe %d, bounces beside Syr Vondam %d, Scribe exile/library fires %d, second-matchup carrier fires %d",
		games, divs, stalls, tl.grave, tl.justice, tl.syrScribe, tl.deathBeside, tl.bounceBesideSyr, tl.scribeExileLib, tl.other)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		what    string
		n, need int
	}{{"graveyard-leave fires", tl.grave, 10}, {"Justice bounce counters", tl.justice, 10},
		{"Syr Vondam / Scribe leaves", tl.syrScribe, 10}, {"deaths beside Justice or Scribe", tl.deathBeside, 5},
		{"bounces beside Syr Vondam", tl.bounceBesideSyr, 5}, {"Scribe exile/library fires", tl.scribeExileLib, 3},
		{"second-matchup carrier fires", tl.other, 10}} {
		if c.n < c.need {
			t.Errorf("only %d %s (need %d)", c.n, c.what, c.need)
		}
	}
}

// TestCzpairsMutation ignores a Leaves line's destination mask: Justice
// fires on a death, gorge's does not, so the games must diverge.
func TestCzpairsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runCzpairs(t, 128, true)
	if divs == 0 {
		t.Fatalf("no-destination mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}

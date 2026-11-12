package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Rebound lockstep (CR 702.88a): a hand-cast instant or sorcery with the
// keyword is exiled as it resolves and, at its controller's next upkeep, may
// be cast from exile without paying its mana cost; that recast resolves to the
// graveyard (it does not rebound again). Both seats play the same deck over
// the generated rebound pool, and the four decks hold every carrier the pool
// lowers, so each is cast and rebounded by both engines. Each deck pairs its
// carriers with what they need: creatures to pump, blink, tap and copy, cheap
// spells for Cascade to find, burn for Fevered Suspicion's opponents and for
// Profound Journey's graveyard.

var reboundDecks = []struct {
	name string
	deck []cardset.Entry
}{
	{"cheap", []cardset.Entry{
		{Name: "Ephemerate", Count: 2}, {Name: "Invisible Force Field", Count: 2},
		{Name: "Transpose", Count: 2}, {Name: "Survival Cache", Count: 2},
		{Name: "Terramorph", Count: 2}, {Name: "Consuming Vapors", Count: 2},
		{Name: "Fantastic Elasticity", Count: 2}, {Name: "Flame On!", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Island", Count: 4},
		{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 3}, {Name: "Forest", Count: 4},
		{Name: "Mountain", Count: 1},
	}},
	{"big", []cardset.Entry{
		{Name: "Quantum Misalignment", Count: 2}, {Name: "Into the Time Vortex", Count: 2},
		{Name: "Nomads' Assembly", Count: 2}, {Name: "Recurring Insight", Count: 1},
		{Name: "It's Clobberin' Time!", Count: 2}, {Name: "Fevered Suspicion", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Island", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 3},
		{Name: "Swamp", Count: 3}, {Name: "Forest", Count: 4},
	}},
	{"pump", []cardset.Entry{
		{Name: "Artful Maneuver", Count: 2}, {Name: "Prey's Vengeance", Count: 2},
		{Name: "Virulent Swipe", Count: 2}, {Name: "Distortion Strike", Count: 2},
		{Name: "Taigam's Strike", Count: 2}, {Name: "Great Teacher's Decree", Count: 2},
		{Name: "Ojutai's Breath", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Plains", Count: 4}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 6},
		{Name: "Swamp", Count: 3},
	}},
	{"value", []cardset.Entry{
		{Name: "Faithless Salvaging", Count: 2}, {Name: "Staggershock", Count: 2},
		{Name: "Sight Beyond Sight", Count: 2}, {Name: "Trumpeting Herd", Count: 2},
		{Name: "Ojutai's Summons", Count: 2}, {Name: "Void Squall", Count: 2},
		{Name: "Profound Journey", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Island", Count: 5}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 5},
		{Name: "Plains", Count: 4},
	}},
}

// reboundMin is the floor on each carrier's hand casts and on its rebound
// recasts, per deck run.
const reboundMin = 3

// reboundCarriers is every printed K:Rebound card the pool lowers (the test
// checks it against the pool table's CardDef.ReboundAb).
var reboundCarriers = map[string]bool{
	"Ephemerate": true, "Invisible Force Field": true, "Transpose": true, "Survival Cache": true,
	"Terramorph": true, "Consuming Vapors": true, "Fantastic Elasticity": true, "Flame On!": true,
	"Quantum Misalignment": true, "Into the Time Vortex": true, "Nomads' Assembly": true,
	"Recurring Insight": true, "It's Clobberin' Time!": true, "Fevered Suspicion": true,
	"Artful Maneuver": true, "Prey's Vengeance": true, "Virulent Swipe": true, "Distortion Strike": true,
	"Taigam's Strike": true, "Great Teacher's Decree": true, "Ojutai's Breath": true,
	"Faithless Salvaging": true, "Staggershock": true, "Sight Beyond Sight": true, "Trumpeting Herd": true,
	"Ojutai's Summons": true, "Void Squall": true, "Profound Journey": true,
}

// reboundBody is the Execute$ name of gorge's Rebound delayed trigger
// (engine/cards/link.go): its DelayedRegister is the promise made as the
// resolved hand-cast spell is exiled, its DelayedPush the upkeep offer.
const reboundBody = "__kwReboundCast"

// reboundScan tallies, from gorge's event log, each carrier's casts from hand
// ("cast"), its Rebound promises ("registered"), the upkeep offers
// ("offered") and the recasts an offer made ("rebound": a cast from exile, in
// that upkeep, of the very card the offer named). A cast from exile by any
// other route (Cascade, Fevered Suspicion's free casts) is an "exile cast",
// never a rebound. "unearned" counts a promise made for a card whose last cast
// was not from the hand (a rebound recast, a cast from exile), and "offbeat"
// an offer outside its controller's upkeep; CR 702.88a makes both 0.
func reboundScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	carrier := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil && reboundCarriers[ob.Face().Name] {
			return ob.Face().Name
		}
		return ""
	}
	var step state.Step
	var active state.PlayerID
	offered := map[state.ObjID]bool{}  // the current upkeep's offers
	fromHand := map[state.ObjID]bool{} // cards whose last cast was from the hand
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			active = ev.Player
		case events.StepChange:
			step = ev.Step
			clear(offered)
		case events.DelayedRegister, events.DelayedPush:
			c := carrier(ev.Obj)
			if ev.Counter != reboundBody || c == "" {
				continue
			}
			if ev.Kind == events.DelayedRegister {
				n[c+" registered"]++
				if !fromHand[ev.Obj] {
					n["unearned"]++
				}
				continue
			}
			n[c+" offered"]++
			if step != state.StepUpkeep || ev.Player != active {
				n["offbeat"]++
				continue
			}
			offered[ev.Obj] = true
		case events.PutOnStack:
			c := carrier(ev.Obj)
			if c == "" {
				continue
			}
			fromHand[ev.Obj] = ev.From == state.ZHand
			switch {
			case ev.From == state.ZHand:
				n[c+" cast"]++
			case ev.From == state.ZExile && offered[ev.Obj]:
				delete(offered, ev.Obj)
				n[c+" rebound"]++
				n["rebound"]++
			case ev.From == state.ZExile:
				n[c+" exile cast"]++
			}
		}
	}
}

func runRebound(t *testing.T, reg *cards.Registry, set cardset.Set, deck []cardset.Entry, seeds uint64) (games int, branch map[string]int) {
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = deck
		for _, e := range deck {
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
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"reb-a", "reb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		reboundScan(gs, branch)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func reboundSummary(branch map[string]int) string {
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	return strings.Join(bs, " ")
}

func TestReboundLockstep(t *testing.T) {
	t.Parallel()

	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rebound")
	if err != nil {
		t.Fatal(err)
	}
	// The precondition of the per-carrier floors below: the pool lowers
	// exactly reboundCarriers' Rebound, and every one is in a deck.
	inDeck := map[string]bool{}
	for _, d := range reboundDecks {
		for _, e := range d.deck {
			inDeck[e.Name] = true
		}
	}
	names := set.Names()
	for i := 0; i < set.NumCards(); i++ {
		if lowered := set.Def(i).ReboundAb != 0; lowered != reboundCarriers[names[i]] {
			t.Errorf("%s: pool table ReboundAb set %v, reboundCarriers %v", names[i], lowered, reboundCarriers[names[i]])
		}
		if reboundCarriers[names[i]] && !inDeck[names[i]] {
			t.Errorf("%s: a lowered carrier in no deck", names[i])
		}
	}
	var want []string
	for c := range reboundCarriers {
		want = append(want, c)
	}
	sort.Strings(want)
	for _, c := range want {
		if _, ok := set.Lookup(c); !ok {
			t.Errorf("%s: not in the rebound pool", c)
		}
	}
	if t.Failed() {
		return
	}

	for _, d := range reboundDecks {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			games, branch := runRebound(t, reg, set, d.deck, 150)
			t.Logf("%d games; %s", games, reboundSummary(branch))
			if t.Failed() {
				return
			}
			for _, e := range d.deck {
				if !reboundCarriers[e.Name] {
					continue
				}
				if n := branch[e.Name+" cast"]; n < reboundMin {
					t.Errorf("%s cast from hand %d times, want at least %d", e.Name, n, reboundMin)
				}
				if n := branch[e.Name+" rebound"]; n < reboundMin {
					t.Errorf("%s recast by its Rebound offer %d times, want at least %d", e.Name, n, reboundMin)
				}
			}
			if branch["unearned"] != 0 || branch["offbeat"] != 0 {
				t.Errorf("%d promises for a card not cast from the hand, %d offers outside the controller's upkeep, want 0 and 0", branch["unearned"], branch["offbeat"])
			}
		})
	}
}

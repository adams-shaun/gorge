package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MoveCounter Any/All lockstep: Nesting Grounds ("move a counter from
// target permanent you control onto another target permanent", CounterType$
// Any -- the kernel asks the kind only when the origin holds more than one)
// and The Ozolith (beginning of combat, may move all counters onto target
// creature, CounterType$ All -- every kind moves with its count, CR 122.5).
// Counter carriers: Karstoderm (five +1/+1, removed on artifact enters),
// Viashino Branchrider (kicked: two +1/+1), Fume Spitter (a -1/-1 on entry)
// and Wickerbough Elder (its -1/-1 pumper). The fixture pool is
// dirac/cardset/pools/fixtures/movecounterany.

// The two-kind carrier gorge's Any path asks about: a +1/+1 and a -1/-1
// counter annihilate each other on any permanent (sba.go's countersPair
// action), so the pair that survives to the ask is a counter carrier that
// also holds a stun counter: gorge's stun never leaves (it only denies the
// untap), so a Fume Spitter its own controller Out Colded keeps a -1/-1 and
// a stun side by side through its main phases -- where Nesting Grounds is
// activated. Both decks carry the Out Cold and the Spitter for that; deck
// 1's Karstoderm +1/+1s and the Elder's -1/-1s feed the Ozolith's
// every-kind move.
var moveCounterAnyDecks = [2][]cardset.Entry{
	{
		{Name: "The Ozolith", Count: 2}, {Name: "Nesting Grounds", Count: 4},
		{Name: "Karstoderm", Count: 2}, {Name: "Viashino Branchrider", Count: 2},
		{Name: "Wickerbough Elder", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Fume Spitter", Count: 4}, {Name: "Out Cold", Count: 4},
		{Name: "Shock", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Mountain", Count: 2},
		{Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "The Ozolith", Count: 3}, {Name: "Nesting Grounds", Count: 4},
		{Name: "Karstoderm", Count: 4}, {Name: "Viashino Branchrider", Count: 4},
		{Name: "Wickerbough Elder", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Fume Spitter", Count: 4}, {Name: "Out Cold", Count: 4},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 2}, {Name: "Mountain", Count: 2},
	},
}

var moveCounterAnyWatch = map[string]bool{
	"The Ozolith": true, "Karstoderm": true, "Viashino Branchrider": true,
	"Wickerbough Elder": true, "Fume Spitter": true, "Out Cold": true,
}

// moveCounterAnyStats records what the moves reached: the gorge side's
// "move_counter_kind" pick (the Any path asked -- the carrier held more than
// one kind), and The Ozolith emptying its counters (the All path ran).
type moveCounterAnyStats struct {
	games, divs, stalls int
	casts               map[string]int
	ngActivations       int // Nesting Grounds activations taken
	kindAsks            int // games in which gorge posed the kind pick
	ozolithMoves        int // games in which the Ozolith emptied counters
	twoKind             int // sightings of a permanent holding two counter kinds
}

// moveCounterAnyPolicy is the fixture watch policy plus two biases: activate
// Nesting Grounds' move-a-counter ability when it can (a land plays as
// itself, so the cast bias alone would never fire the Any path), and answer
// a target ask with a permanent holding two counter kinds when one is
// offered (the random pick never lands a two-kind origin, so gorge's kind
// pick would never be posed).
func moveCounterAnyPolicy(seed uint64, watch map[string]bool, casts map[string]int, acts *int) Policy {
	base := fixtureWatchPolicy(seed, watch, casts)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 && ownerHasTwoKind(c.G, c.D.Player) {
			// Save the Nesting Grounds tap for the moment it pays off: the
			// activating player controls a permanent holding two counter
			// kinds, so its origin ask can offer gorge's kind pick (the
			// unconditional activation spent every tap before the stun
			// carrier was ever cast).
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate && refCardName(c.G, a.Src()) == "Nesting Grounds" {
					*acts++
					return a
				}
			}
		}
		if c.G != nil && c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				if twoKindTarget(c.G, a) {
					return a
				}
			}
			// Out Cold's enchant ask: attach it to one of the activating
			// player's own counter carriers (their -1/-1'd Fume Spitter, a
			// +1/+1 Karstoderm), so the stun lands beside counters that
			// survive and the permanent is a two-kind origin from its
			// first moment -- and gorge's stun keeps it there.
			if pd := c.G.Pending(); pd != nil && refCardName(c.G, c.G.InstRef(pd.Source)) == "Out Cold" {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Controller == state.PlayerID(c.D.Player) &&
							ctrTotal(ob) > 0 {
							return a
						}
					}
				}
			}
		}
		return base.Pick(c)
	})
}

// ownerHasTwoKind reports whether the seat controls a battlefield permanent
// holding counters of two or more kinds.
func ownerHasTwoKind(g *GorgeSide, player uint8) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(player)) {
		if o := g.E.G.Obj(id); o != nil && counterKinds(o) >= 2 {
			return true
		}
	}
	return false
}

// counterKinds counts the kinds of real counters an object holds (gorge's
// internal status markers excluded).
func counterKinds(o *state.Object) int {
	n := 0
	for i := range o.Counters {
		if o.Counters[i].N > 0 && !state.InternalCounterMarker(o.Counters[i].Kind) {
			n++
		}
	}
	return n
}

// twoKindTarget reports whether the target option names a battlefield
// permanent holding counters of two or more kinds: the origin that makes
// gorge's effMoveCounter CounterType$ Any pose its kind pick.
func twoKindTarget(g *GorgeSide, a Canon) bool {
	if a.Kind() != ActTarget {
		return false
	}
	o, ok := g.GorgeOption(a)
	if !ok || o.Obj == 0 {
		return false
	}
	ob := g.E.G.Obj(o.Obj)
	return ob != nil && counterKinds(ob) >= 2
}

// ctrTotal is the counters an object holds, excluding gorge's internal status
// markers (state.InternalCounterMarker): the same census gorge's every-kind
// walk makes.
func ctrTotal(o *state.Object) int {
	n := 0
	for i := range o.Counters {
		if o.Counters[i].N > 0 && !state.InternalCounterMarker(o.Counters[i].Kind) {
			n += int(o.Counters[i].N)
		}
	}
	return n
}

// moveCounterAnyCheck watches The Ozolith's counter total across the game: a
// drop to zero after holding counters is the begin-combat move having run.
func moveCounterAnyCheck(ozN map[state.ObjID]int, moved *bool, twoKind *int, seenKinds map[string]int) func(*GorgeSide, Side) *Divergence {
	return func(ref *GorgeSide, _ Side) *Divergence {
		if d := ref.E.Pending(); d != nil && d.ResumeKind == "move_counter_kind" {
			*twoKind += 1000000 // the Any path asked (debug probe)
		}
		g := ref.E.G
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				o := g.Obj(id)
				if o == nil {
					continue
				}
				if counterKinds(o) >= 2 {
					*twoKind++
					var ks []string
					for i := range o.Counters {
						if o.Counters[i].N > 0 && !state.InternalCounterMarker(o.Counters[i].Kind) {
							ks = append(ks, o.Counters[i].Kind)
						}
					}
					seenKinds[strings.Join(ks, "+")]++
				}
				if o.Face() == nil || o.Face().Name != "The Ozolith" {
					continue
				}
				n := ctrTotal(o)
				if prev := ozN[id]; prev > 0 && n == 0 {
					*moved = true
				}
				ozN[id] = n
			}
		}
		return nil
	}
}

func runMoveCounterAny(t *testing.T, seeds int, mutate func(*kop.Table)) moveCounterAnyStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("movecounterany")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range moveCounterAnyDecks {
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
	tbl, _, _, err := set.Matchup(moveCounterAnyDecks[0], moveCounterAnyDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	st := moveCounterAnyStats{casts: map[string]int{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mvca-a", "mvca-b"}, decks, 1)
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
		ozN := map[state.ObjID]int{}
		moved := false
		seenKinds := map[string]int{}
		pol := moveCounterAnyPolicy(seed, moveCounterAnyWatch, st.casts, &st.ngActivations)
		// The kind pick is the only choose:counter election this pool poses
		// (every other counter removal names its kind): see its options once
		// a game and the Any path asked.
		kindPick := counterPickPolicy(pol, &st.kindAsks)
		res := LockstepLed(m, seed, g, k, kindPick, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30,
			Check: moveCounterAnyCheck(ozN, &moved, &st.twoKind, seenKinds)})
		CloseSide(k)
		if moved {
			st.ozolithMoves++
		}
		for k, n := range seenKinds {
			t.Logf("seed %d two-kind %s x%d", seed, k, n)
		}
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			if mutate == nil {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v",
					seed, d.At, strings.TrimSpace(d.What+" "+d.Field+" "+d.Detail), d.Diff, d.RefOptions, d.KerOptions)
			}
		}
	}
	return st
}

// counterPickPolicy wraps a policy and counts (through asked) the first game
// in which a choose:counter election -- gorge's "move_counter_kind" pick --
// is posed.
func counterPickPolicy(base Policy, asked *int) Policy {
	seen := false
	return PolicyFunc(func(c *PickCtx) Canon {
		if !seen {
			code, _ := chooseCode("counter")
			for _, a := range c.D.Options {
				if a.Kind() == ActChoose+CanonKind(code) {
					seen = true
					*asked++
					break
				}
			}
		}
		return base.Pick(c)
	})
}

func TestMoveCounterAnyAllLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runMoveCounterAny(t, 64, nil)
	t.Logf("%d games, %d divergences, %d stalls; casts %v; Nesting Grounds activations %d; kind picks %d games, Ozolith emptied %d games, two-kind sightings %d",
		st.games, st.divs, st.stalls, st.casts, st.ngActivations, st.kindAsks, st.ozolithMoves, st.twoKind)
	if st.divs != 0 {
		t.Fatalf("%d divergences", st.divs)
	}
	if st.stalls != 0 {
		t.Fatalf("%d kernel stalls", st.stalls)
	}
	if st.casts["The Ozolith"] < 1 {
		t.Fatalf("casts %v: want the Ozolith played", st.casts)
	}
	if st.kindAsks == 0 || st.ozolithMoves == 0 {
		t.Fatalf("no counter moved: kind picks %d, Ozolith emptied %d", st.kindAsks, st.ozolithMoves)
	}
}

// TestMoveCounterAnyAllMutation: swapping each move's destination for its
// source must make some game diverge.
func TestMoveCounterAnyAllMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mu := range []struct {
		name string
		card string
		op   kop.Op
	}{
		{"Nesting Grounds kind move", "Nesting Grounds", kop.OpMoveCounterKind},
		{"Ozolith all-kinds move", "The Ozolith", kop.OpMoveCounterAll},
	} {
		st := runMoveCounterAny(t, 64, func(tbl *kop.Table) {
			n := mutateCard(t, tbl, mu.card, func(a *kop.Ability, in *kop.Instr) {
				if in.Op == mu.op {
					in.Obj = kop.Ref(in.C) // the destination becomes the origin: the kernel moves nothing where gorge moves
				}
			})
			if n == 0 {
				t.Fatalf("no %s op in the lowered %s", mu.name, mu.card)
			}
		})
		if st.divs == 0 {
			t.Fatalf("%s: mutation ran %d games with no divergence", mu.name, st.games)
		}
		t.Logf("%s: diverged in %d of %d games", mu.name, st.divs, st.games)
	}
}

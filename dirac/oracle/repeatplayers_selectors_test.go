package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// The RepeatEach RepeatPlayers$ selectors kgen lowers in
// lower_repeatselectors.go, over the repeatplayerssel fixture pool
// (dirac/cardset/pools/fixtures/repeatplayerssel):
//
//   - Keen Duelist: TargetedAndYou, the controller's loop then the targeted
//     opponent's (gorge repeatPlayers walks AliveFrom(controller)); each loses
//     life equal to the mana value the other revealed.
//   - Skull Storm: OppNonRememberedController, each opponent who sacrificed
//     nothing loses half their life, rounded up (the kernel's
//     RefOppNonRemCtrl).
//
// Every game starts from a fixed board: seat 0 holds Keen Duelist and nine
// Swamps on the battlefield and Skull Storm in hand, so both loops resolve in
// the first turns.

var repeatSelDecks = [2][]cardset.Entry{
	{{Name: "Keen Duelist", Count: 4}, {Name: "Skull Storm", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 28}},
	{{Name: "Grizzly Bears", Count: 6}, {Name: "Sewer Rats", Count: 6}, {Name: "Keen Duelist", Count: 4}, {Name: "Swamp", Count: 12}, {Name: "Forest", Count: 12}},
}

// repeatSelStart is seat 0's fixed board (deck indices of repeatSelDecks[0])
// and seat 1's: a creature on its battlefield for odd seeds, so Skull Storm's
// loop both skips and visits the opponent across the seeds.
func repeatSelStart(seed uint64) *rules.StartSpec {
	s0 := rules.StartSeat{Battlefield: []int{0, 12, 13, 14, 15, 16, 17, 18, 19, 20}, Hand: []int{4, 5, 8, 21}}
	s1 := rules.StartSeat{Battlefield: []int{16, 17, 28}, Hand: []int{6, 12, 29}}
	if seed%2 == 1 {
		s1.Battlefield = append(s1.Battlefield, 0)
	}
	return &rules.StartSpec{Seats: []rules.StartSeat{s0, s1}}
}

// repeatSelStats are the preconditions read off gorge's logs.
type repeatSelStats struct {
	duel  int // Keen Duelist resolutions in which both players lost life
	storm int // Skull Storm resolutions in which some opponent lost life
}

// repeatSelLosses lists, per resolution of card (a spell's Resolve, or a
// trigger the card put on the stack), the players who lost life in it, in
// event order.
func repeatSelLosses(g *GorgeSide, card string) [][]state.PlayerID {
	var out [][]state.PlayerID
	armed, open := false, false
	var cur []state.PlayerID
	for j := range g.E.L.Events {
		e := &g.E.L.Events[j]
		switch {
		case e.Kind == events.TriggerPush:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == card {
				armed = true
			}
		case e.Kind == events.Resolve && (armed || e.Text == card):
			armed, open, cur = false, true, nil
		case open && e.Kind == events.LifeChange && e.Amount < 0:
			cur = append(cur, e.Player)
		case open && e.Kind == events.Priority:
			open = false
			if len(cur) > 0 {
				out = append(out, cur)
			}
		}
	}
	return out
}

// runRepeatSel plays seeds 1..seeds of the fixture from repeatSelStart, gorge
// leading, Keen Duelist and Skull Storm cast when offered. mutate rewrites
// the table first.
func runRepeatSel(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) (fixtureRun, repeatSelStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("repeatplayerssel")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range repeatSelDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(repeatSelDecks[0], repeatSelDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	watch := map[string]bool{"Keen Duelist": true, "Skull Storm": true}
	run := fixtureRun{casts: map[string]int{}}
	var st repeatSelStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"repeatsel-a", "repeatsel-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		m.Start = repeatSelStart(seed)
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, watch, run.casts), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		run.games++
		for _, l := range repeatSelLosses(g, "Keen Duelist") {
			if len(l) == 2 {
				st.duel++
			}
		}
		st.storm += len(repeatSelLosses(g, "Skull Storm"))
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, st
}

// TestRepeatPlayersSelectorsLockstep: 64 seeds, no divergence, no kernel
// stall, and both loops resolved with a life loss.
func TestRepeatPlayersSelectorsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run, st := runRepeatSel(t, 64, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, %+v", run.games, run.divs, run.stalls, st)
	if st.duel == 0 {
		t.Error("no Keen Duelist resolution made both players lose life: the TargetedAndYou loop is not reached")
	}
	if st.storm == 0 {
		t.Error("no Skull Storm resolution made an opponent lose life: the OppNonRememberedController loop is not reached")
	}
}

// TestRepeatPlayersSelectorsMutation: with each selector swapped for the
// controller alone, some game diverges.
func TestRepeatPlayersSelectorsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range []struct {
		card string
		from kop.Ref
	}{
		{"Keen Duelist", kop.RefTarget0},
		{"Skull Storm", kop.RefOppNonRemCtrl},
	} {
		t.Run(strings.ReplaceAll(c.card, " ", ""), func(t *testing.T) {
			run, _ := runRepeatSel(t, 16, false, func(tbl *kop.Table) {
				n := mutateCard(t, tbl, c.card, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpRepeatEachPlayer && in.Obj == c.from {
						in.Obj = kop.RefController
					}
				})
				if n == 0 {
					t.Fatalf("%s: no RepeatEachPlayer over %v in the table", c.card, c.from)
				}
			})
			run.requireDivergence(t, c.card+"'s loop over the controller alone")
		})
	}
	t.Run("DisorientingChoice", requireDCControllerMutationDiverges) // repeatplayers_tgtctrl_test.go
}

// TestFourSeatRepeatPlayersSelectors: a four-seat pod from a fixed start,
// seat 0 casting Skull Storm off nine Swamps. With no opponent able to
// sacrifice, the loop visits seats 1, 2 and 3 in that order (gorge's
// AliveFrom(controller)); with seat 2 holding a creature it visits 1 and 3,
// and reading every opponent regardless of the remembered set
// (dirac.DebugOppNonRemFirstOnly) diverges. The steal case has seat 2 take
// seat 1's creature with Control Magic and then sacrifice it: the loop must
// exclude seat 2 by the creature's last-known controller (CR 608.2h), and the
// live-controller read (dirac.DebugOppNonRemLive) must diverge. Disorienting
// Choice's TargetedController loop visits its pod in the same order.
func TestFourSeatRepeatPlayersSelectors(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("DisorientingChoice", requireDCFourSeatVisit) // repeatplayers_tgtctrl_test.go
	storm := append(deckOf(4, "Skull Storm"), deckOf(36, "Swamp")...)
	// stormLate holds Skull Storm with seven Swamps down and two in hand:
	// turn 1 cannot cast it, turn 5 (after seat 2's turn 3) can.
	stormLate := append(deckOf(4, "Skull Storm"), deckOf(36, "Swamp")...)
	lands := deckOf(40, "Swamp")
	islands := deckOf(40, "Island")
	bears := append(deckOf(1, "Grizzly Bears"), deckOf(39, "Swamp")...)
	cmagic := append(deckOf(4, "Control Magic"), deckOf(36, "Island")...)
	s0 := rules.StartSeat{Battlefield: []int{4, 5, 6, 7, 8, 9, 10, 11, 12}, Hand: []int{0, 13}}
	s0Late := rules.StartSeat{Battlefield: []int{4, 5, 6, 7, 8, 9, 10}, Hand: []int{0, 11, 12}}
	land := rules.StartSeat{Battlefield: []int{0}, Hand: []int{1, 2}}
	withBear := rules.StartSeat{Battlefield: []int{0, 1}, Hand: []int{2, 3}}
	cmStart := rules.StartSeat{Battlefield: []int{4, 5, 6, 7}, Hand: []int{0, 8}}
	for _, c := range []struct {
		name  string
		decks [4][]string
		start []rules.StartSeat
		want  []state.PlayerID
		watch map[string]bool
		steal bool
		mut   func()
	}{
		{"all three", [4][]string{storm, lands, lands, lands}, []rules.StartSeat{s0, land, land, land}, []state.PlayerID{1, 2, 3}, nil, false, nil},
		{"seat 2 sacrifices", [4][]string{storm, lands, bears, lands}, []rules.StartSeat{s0, land, withBear, land}, []state.PlayerID{1, 3}, nil, false, func() { dirac.DebugOppNonRemFirstOnly = true }},
		{"stolen sacrifice", [4][]string{stormLate, bears, cmagic, islands}, []rules.StartSeat{s0Late, withBear, cmStart, land}, []state.PlayerID{1, 3}, map[string]bool{"Skull Storm": true, "Control Magic": true}, true, func() { dirac.DebugOppNonRemLive = true }},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", ""), func(t *testing.T) {
			play := repeatSelFourSeat(t, c.decks, c.start, c.watch, c.steal)
			seen := 0
			for seed := uint64(1); seed <= 4; seed++ {
				res, g := play(seed)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
				}
				if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
				}
				for _, l := range repeatSelLosses(g, "Skull Storm") {
					if !slices.Equal(l, c.want) {
						t.Errorf("seed %d: Skull Storm's loop visited %v, want %v", seed, l, c.want)
					}
					seen++
				}
			}
			if seen == 0 {
				t.Fatal("Skull Storm's loop never resolved")
			}
			if c.mut == nil {
				return
			}
			c.mut()
			defer func() { dirac.DebugOppNonRemFirstOnly, dirac.DebugOppNonRemLive = false, false }()
			for seed := uint64(1); seed <= 4; seed++ {
				if res, _ := play(seed); res.Div != nil {
					return
				}
			}
			if c.steal {
				t.Fatal("reading the remembered controllers live changed no game: no opponent sacrificed a stolen creature")
			}
			t.Fatal("ignoring the remembered set changed no game: the loop's set is not read")
		})
	}
}

// castAlwaysPolicy casts the watched cards whenever priority offers one with
// an empty stack, and otherwise plays the random policy. The steal fixture
// needs Control Magic and Skull Storm to land in a fixed order; the watch
// policy's 1-in-4 skip would leave some seeds without the stolen sacrifice.
func castAlwaysPolicy(seed uint64, watch map[string]bool) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if watch[refCardName(c.G, a.Src())] {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// repeatSelFourSeat returns a player of four-seat pods of decks on the
// fixture pool from start, dirac against gorge, the watched cards cast when
// offered, one seed per call.
func repeatSelFourSeat(t *testing.T, decks [4][]string, start []rules.StartSeat, watch map[string]bool, always bool) func(seed uint64) (Result, *GorgeSide) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("repeatplayerssel")
	if err != nil {
		t.Fatal(err)
	}
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return func(seed uint64) (Result, *GorgeSide) {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		m.Start = &rules.StartSpec{Seats: start}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 1500, 6
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		if watch == nil {
			watch = map[string]bool{"Skull Storm": true}
		}
		var pol Policy
		if always {
			pol = castAlwaysPolicy(seed, watch)
		} else {
			pol = fixtureWatchPolicy(seed, watch, map[string]int{})
		}
		return LockstepLed(m, seed, g, k, pol, rc), g
	}
}

// dcLandSearches lists, per resolution of Disorienting Choice, the lands the
// X-counted library search put onto the battlefield: the MoveZone
// library->battlefield events between the spell's Resolve and the next
// priority. ChangeNum$ X reads the spell's runtime store (DBIncrease's
// X/Plus.1 write), so one kept permanent means exactly one searched land.
func dcLandSearches(g *GorgeSide) []int {
	var out []int
	open := false
	n := 0
	for j := range g.E.L.Events {
		e := &g.E.L.Events[j]
		switch {
		case e.Kind == events.Resolve && e.Text == "Disorienting Choice":
			open, n = true, 0
		case open && e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZBattlefield:
			n++
		case open && e.Kind == events.Priority:
			open = false
			out = append(out, n)
		}
	}
	return out
}

// dcTargetedKeepPolicy casts Disorienting Choice and Regrowth whenever
// priority offers one with an empty stack, always answers the target asks with
// the offered Ornithopter (the Disorienting Choice target) or Disorienting
// Choice itself (Regrowth's graveyard pick), and always answers the
// per-permanent GenericChoice with "Keep targeted card" (counted into *kept)
// and always takes the offered Forest from the land search. Every Disorienting
// Choice cast therefore keeps its one chosen permanent and stores X = 1 on the
// spell object.
func dcTargetedKeepPolicy(seed uint64, kept *int) Policy {
	base := castAlwaysPolicy(seed, map[string]bool{"Disorienting Choice": true, "Regrowth": true})
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if c.D.Kind == DTarget {
				for _, o := range c.D.Options {
					oa, ok := c.G.GorgeOption(o)
					if !ok || oa.Obj == 0 {
						continue
					}
					if ob := c.G.E.G.Obj(oa.Obj); ob != nil && ob.Face() != nil &&
						(ob.Face().Name == "Ornithopter" || ob.Face().Name == "Disorienting Choice") {
						return o
					}
				}
			}
			for _, o := range c.D.Options {
				if oa, ok := c.G.GorgeOption(o); ok && oa.Kind == "mode" && oa.Label == "Keep targeted card" {
					*kept++
					return o
				}
			}
			if c.D.Kind == DChoose {
				// the X-counted library search's pick: always take the offered
				// Forest (its "up to X" ask would let the random base search
				// no land and hide the X read)
				for _, o := range c.D.Options {
					if oa, ok := c.G.GorgeOption(o); ok && oa.Obj != 0 {
						if ob := c.G.E.G.Obj(oa.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Forest" {
							return o
						}
					}
				}
			}
		}
		return base.Pick(c)
	})
}

// TestDisorientingChoiceLockstep: the TargetedController carrier runs decision
// for decision, deterministically pinning the CR 400.7 spell-store reset. Seat
// 0 holds one copy of Disorienting Choice, one Regrowth and eight Forests
// down; seat 1 two Ornithopters. The policy names an Ornithopter on every
// cast and keeps the chosen permanent every time, so each cast stores X = 1 on
// the spell object (DBIncrease) and searches exactly one land (ChangeNum$ X).
// Regrowth returns the same Disorienting Choice object from the graveyard to
// hand and the policy casts it again: the second cast is the same object, so
// it only starts from a clean store because the stack exit cleared it -- gorge
// foldMoveZone's RuntimeSVars clear and the kernel's numStackExit.
// A reverted clear makes the second cast read X = 1, tally to X = 2 and search
// two lands, failing the count and the lockstep alike.
func TestDisorientingChoiceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("repeatplayerssel")
	if err != nil {
		t.Fatal(err)
	}
	lists := [2][]cardset.Entry{
		{{Name: "Disorienting Choice", Count: 1}, {Name: "Regrowth", Count: 1}, {Name: "Forest", Count: 38}},
		{{Name: "Ornithopter", Count: 4}, {Name: "Forest", Count: 36}},
	}
	var decks [2][]*cards.Card
	for s, d := range lists {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(lists[0], lists[1])
	if err != nil {
		t.Fatal(err)
	}
	start := &rules.StartSpec{Seats: []rules.StartSeat{
		{Battlefield: []int{2, 3, 4, 5, 6, 7, 8, 9}, Hand: []int{0, 1}},
		{Battlefield: []int{0, 1, 2, 3}, Hand: []int{4, 5}},
	}}
	for seed := uint64(1); seed <= 3; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"dc-a", "dc-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		m.Start = start
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		kept := 0
		res := LockstepLed(m, seed, g, k, dcTargetedKeepPolicy(seed, &kept),
			RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 10})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: diverged at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if kept < 2 {
			t.Fatalf("seed %d: only %d keeps answered: the carrier never reached the GenericChoice twice", seed, kept)
		}
		searches := dcLandSearches(g)
		if len(searches) != 2 {
			t.Fatalf("seed %d: %d Disorienting Choice resolutions %v, want 2 (cast, Regrowth back to hand, recast)", seed, len(searches), searches)
		}
		for i, n := range searches {
			if n != 1 {
				t.Errorf("seed %d: resolution %d searched %d lands, want 1: ChangeNum$ X must start from the stack-exit-cleared store (CR 400.7)", seed, i+1, n)
			}
		}
	}
}

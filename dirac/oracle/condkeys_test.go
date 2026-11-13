package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// condkeys pool, whose carriers are Eye of Jace (a ConditionZone$ Graveyard
// presence gate on its three node effects: sacrifice, 2 damage to each
// opponent, gain 2 life), The Rollercrusher Ride (Delirium$ True on a
// noncombat DamageDone doubler) and Putrid Pals (Revolt$ True on its
// enters-with-two-+1/+1-counters replacement). Thought Scour, Armored Skaab
// and Mulch fill the graveyard (7 cards and 4 card types), Shock and
// Lightning Bolt are the burn Rollercrusher doubles, Viscera Seer sacrifices
// a creature for revolt, and the vanilla bodies give both seats a board.
//
// Every counter is read from gorge's own side: the outcome of each Eye of
// Jace upkeep resolution (was the artifact sacrificed?), the damage amount of
// each resolved Shock / Lightning Bolt, and the +1/+1 counters a Putrid Pals
// permanent has when it first appears on the battlefield.

var kcondkeysDecks = [2][]cardset.Entry{
	{
		{Name: "Eye of Jace", Count: 4}, {Name: "The Rollercrusher Ride", Count: 1}, {Name: "Putrid Pals", Count: 3},
		{Name: "Thought Scour", Count: 4}, {Name: "Armored Skaab", Count: 2}, {Name: "Viscera Seer", Count: 2},
		{Name: "Shock", Count: 4}, {Name: "Opt", Count: 3},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Mulch", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 8},
	},
}

// kcondkeysStats is what gorge's event log says each gate decided.
type kcondkeysStats struct {
	eyeTrue   int // Eye upkeep resolutions where the artifact was sacrificed (gate true)
	eyeFalse  int // ... where it stayed on the battlefield (gate false)
	rollerDbl int // resolved Shock / Lightning Bolt dealt double damage
	burnPlain int // ... dealt plain damage while a Rollercrusher was on the battlefield (no delirium)
	ppWith    int // Putrid Pals entries with its two +1/+1 counters
	ppWithout int // ... without them
}

type kcondkeysStk struct {
	obj    state.ObjID
	trig   bool
	player state.PlayerID
}

type kcondkeysTally struct {
	evAt       int
	stack      []kcondkeysStk
	pend       int // index of the Resolve being evaluated; -1 none
	pendObj    state.ObjID
	pendTrig   bool
	pendName   string
	pendPlayer state.PlayerID
	pendRoller bool
	seenPP     []state.ObjID
	stats      kcondkeysStats
}

func (t *kcondkeysTally) seen(id state.ObjID) bool {
	for _, s := range t.seenPP {
		if s == id {
			return true
		}
	}
	return false
}

// finish evaluates the events of one resolution (from just after its Resolve
// to the next priority boundary).
func (t *kcondkeysTally) finish(evs []events.Event, at int) {
	if t.pend < 0 || t.pend+1 > at {
		t.pend = -1
		return
	}
	win := evs[t.pend+1 : at]
	switch {
	case t.pendTrig && t.pendName == "Eye of Jace":
		sacrificed := false
		for _, w := range win {
			if w.Kind == events.MoveZone && w.Obj == t.pendObj && w.From == state.ZBattlefield && w.To == state.ZGraveyard {
				sacrificed = true
			}
		}
		if sacrificed {
			t.stats.eyeTrue++
		} else {
			t.stats.eyeFalse++
		}
	case !t.pendTrig && (t.pendName == "Shock" || t.pendName == "Lightning Bolt"):
		base := int32(2)
		if t.pendName == "Lightning Bolt" {
			base = 3
		}
		dmg := int32(-1)
		for _, w := range win {
			if w.Kind == events.Damage {
				dmg = w.Amount
				break
			}
		}
		switch dmg {
		case base * 2:
			t.stats.rollerDbl++
		case base:
			if t.pendRoller {
				t.stats.burnPlain++
			}
		}
	}
	t.pend = -1
}

// scanPals records each Putrid Pals permanent the first time it is seen on
// the battlefield, with the counters it entered with.
func (t *kcondkeysTally) scanPals(g *state.Game) {
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if chName(g, id) != "Putrid Pals" || t.seen(id) {
				continue
			}
			t.seenPP = append(t.seenPP, id)
			o := g.Obj(id)
			if o != nil && o.Counter("P1P1") >= 2 {
				t.stats.ppWith++
			} else {
				t.stats.ppWithout++
			}
		}
	}
}

func (t *kcondkeysTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	for t.evAt < len(evs) {
		e := &evs[t.evAt]
		switch e.Kind {
		case events.PutOnStack:
			t.stack = append(t.stack, kcondkeysStk{obj: e.Obj, player: e.Player})
		case events.TriggerPush, events.AbilityPush:
			t.stack = append(t.stack, kcondkeysStk{obj: e.Obj, trig: true, player: e.Player})
		case events.Resolve:
			var top kcondkeysStk
			if e.Text == "" {
				if len(t.stack) > 0 {
					top = t.stack[len(t.stack)-1]
					t.stack = t.stack[:len(t.stack)-1]
				}
			} else {
				for len(t.stack) > 0 && t.stack[len(t.stack)-1].obj != e.Obj {
					t.stack = t.stack[:len(t.stack)-1]
				}
				if len(t.stack) > 0 {
					top = t.stack[len(t.stack)-1]
					t.stack = t.stack[:len(t.stack)-1]
				}
			}
			t.pend = t.evAt
			t.pendObj, t.pendTrig, t.pendPlayer = top.obj, top.trig, top.player
			t.pendName = chName(g, top.obj)
			t.pendRoller = controlsBattlefieldNamed(g, top.player, "The Rollercrusher Ride")
		case events.Priority, events.StepChange, events.TurnChange:
			t.finish(evs, t.evAt)
		}
		t.evAt++
	}
	t.scanPals(g)
	return nil
}

// finishGame closes a game whose last resolution never reached a priority
// boundary.
func (t *kcondkeysTally) finishGame(ref *GorgeSide) {
	t.finish(ref.E.L.Events, len(ref.E.L.Events))
}

// kcondkeysRun plays seeds 1..seeds with both seatings (192 games for 96
// seeds). mutate turns on the kernel's DebugCondKeysIgnore hook.
func kcondkeysRun(t *testing.T, seeds uint64, mutate bool) (games, divs, stalls int, st kcondkeysStats) {
	t.Helper()
	reg, set := kcondkeysSetup(t)
	old := dirac.DebugCondKeysIgnore
	dirac.DebugCondKeysIgnore = mutate
	defer func() { dirac.DebugCondKeysIgnore = old }()
	for swap := 0; swap < 2; swap++ {
		md := kcondkeysDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kcondkeysDecks[1], kcondkeysDecks[0]}
		}
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcondkeys-a", "kcondkeys-b"}, decks, 1)
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
			var tl kcondkeysTally
			tl.pend = -1
			res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{
				Config: cfg, MaxDecisions: 4000, MaxTurns: 20,
				Check:  tl.check,
				Finish: tl.finishGame,
			})
			CloseSide(k)
			games++
			st.eyeTrue += tl.stats.eyeTrue
			st.eyeFalse += tl.stats.eyeFalse
			st.rollerDbl += tl.stats.rollerDbl
			st.burnPlain += tl.stats.burnPlain
			st.ppWith += tl.stats.ppWith
			st.ppWithout += tl.stats.ppWithout
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					lo := max(d.At-20, 0)
					t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func kcondkeysSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("condkeys")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestCondkeysLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 192 seeds per seating: 384 games
	games, divs, stalls, st := kcondkeysRun(t, 192, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("Eye of Jace upkeep resolutions: gate true %d, gate false %d; Rollercrusher doublings %d, plain burn with a Rollercrusher %d; Putrid Pals entries with counters %d, without %d",
		st.eyeTrue, st.eyeFalse, st.rollerDbl, st.burnPlain, st.ppWith, st.ppWithout)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Eye of Jace gate true", st.eyeTrue},
		{"Eye of Jace gate false", st.eyeFalse},
		{"Rollercrusher doublings", st.rollerDbl},
		{"burn not doubled (no delirium)", st.burnPlain},
		{"Putrid Pals entries with counters", st.ppWith},
		{"Putrid Pals entries without counters", st.ppWithout},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestCondkeysMutation turns on DebugCondKeysIgnore: the kernel applies the
// Eye of Jace, Rollercrusher and Putrid Pals gates even when gorge's gate is
// false, so some game must diverge.
func TestCondkeysMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	_, divs, _, _ := kcondkeysRun(t, 64, true)
	if divs == 0 {
		t.Fatalf("DebugCondKeysIgnore: no divergence in 64 seeds")
	}
	t.Logf("%d divergences", divs)
}

// kcondkeysLegendDecks is kcondkeysDecks with a SECOND The Rollercrusher Ride
// and one fewer Thought Scour, still 40 cards. Two copies make the legend rule
// matter: the just-entered copy is the keep answer's loser, events.Move clears
// its live X, and the ETB trigger must still read the paid X off its fire-time
// capture (CR 107.3m).
func kcondkeysLegendDecks() [2][]cardset.Entry {
	md := kcondkeysDecks
	first := append([]cardset.Entry(nil), md[0]...)
	for i := range first {
		switch first[i].Name {
		case "The Rollercrusher Ride":
			first[i].Count = 2
		case "Thought Scour":
			first[i].Count = 3
		}
	}
	return [2][]cardset.Entry{first, md[1]}
}

// kcondkeysLegendAsk is one target decision gorge posed, named after the object
// the ask resolves for (the trigger wrapper's source when the wrapper itself
// carries no face, exactly targetBoundCtx's own read).
type kcondkeysLegendAsk struct {
	name     string
	min, max int
	// departed is true when the ask's source is no longer on the battlefield
	// or the stack: the legend-rule case, where events.Move has already
	// cleared its live X (a normal ETB asks with the permanent still on the
	// battlefield, which would not exercise CR 107.3m's capture).
	departed bool
	// kernel is true when the kernel side is asking for a target at the same
	// decision (the lockstep has already proved the option lists equal).
	kernel bool
}

func kcondkeysLegendAskName(g *state.Game, id state.ObjID) string {
	o := g.Obj(id)
	if o == nil {
		return ""
	}
	if o.Face() != nil {
		return o.Face().Name
	}
	return chName(g, o.Source)
}

type kcondkeysLegendTally struct {
	asks []kcondkeysLegendAsk
}

func (t *kcondkeysLegendTally) check(ref *GorgeSide, k Side) *Divergence {
	d := ref.Pending()
	if d == nil || d.Kind != decision.KTarget {
		return nil
	}
	departed := false
	if o := ref.E.G.Obj(d.Source); o != nil {
		if src := ref.E.G.Obj(o.Source); src != nil {
			departed = src.Zone != state.ZBattlefield && src.Zone != state.ZStack
		}
	}
	r, isKernel := k.(*realSide)
	t.asks = append(t.asks, kcondkeysLegendAsk{
		name:     kcondkeysLegendAskName(ref.E.G, d.Source),
		min:      d.Min,
		max:      d.Max,
		departed: departed,
		kernel:   isKernel && r.d.Kind == dirac.DkTarget,
	})
	return nil
}

// TestCondkeysLegendEtbXBoundLockstep pins the four seeds that used to
// diverge after a second The Rollercrusher Ride was binned by the legend rule
// (CR 704.5j): gorge posed no target ask for the ETB trigger while the kernel
// asked for up to X, because the placement bound read the source permanent's
// live (now cleared) X instead of the spell's paid X (CR 107.3m). The gorge
// pin e801ad31d carries the fire-time fallback (target_ask.go seeds
// resolvedTargetBounds with TriggerContext.TriggerPaidX); this test fails if it
// regresses, not the kernel.
//
// Each seed's paid X is the bound the ask must carry: seed 13 cast X = 1, the
// other three X = 2. The ask must be the legend-rule case (the trigger's
// source already in the graveyard, not a normal ETB ask) and both sides must
// pose it; otherwise a game that never reaches the second legend would pass
// vacuously.
func TestCondkeysLegendEtbXBoundLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := kcondkeysSetup(t)
	md := kcondkeysLegendDecks()
	swapped := [2][]cardset.Entry{md[1], md[0]}
	decks := kcastDeck(t, reg, md)
	decksSwapped := kcastDeck(t, reg, swapped)
	cases := []struct {
		swap int
		seed uint64
		paid int
	}{
		{0, 13, 1},
		{0, 17, 2},
		{1, 43, 2},
		{1, 45, 2},
	}
	for _, tc := range cases {
		td, dd := md, decks
		if tc.swap == 1 {
			td, dd = swapped, decksSwapped
		}
		tbl, _, _, err := set.Matchup(td[0], td[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: tc.seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcondkeys-a", "kcondkeys-b"}, dd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, tc.seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, tc.seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		tl := &kcondkeysLegendTally{}
		res := LockstepLed(m, tc.seed, g, k, RandomPolicy(tc.seed), RunConfig{
			Config: cfg, MaxDecisions: 4000, MaxTurns: 20,
			Check: tl.check,
		})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %s", tc.swap, tc.seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		}
		// A turn-cap end is ordinary for these long games; only the kernel's
		// fixed capacity is a failure, as in kcondkeysRun.
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("swap %d seed %d: kernel stall: %s", tc.swap, tc.seed, res.Stall)
		}
		t.Logf("swap %d seed %d: %d decisions, target asks %v", tc.swap, tc.seed, res.Decisions, tl.asks)
		found := false
		for _, a := range tl.asks {
			if a.name == "The Rollercrusher Ride" && a.max == tc.paid && a.departed && a.kernel {
				found = true
			}
		}
		if !found {
			t.Errorf("swap %d seed %d: no departed-source The Rollercrusher Ride target ask with max %d on both sides (CR 107.3m); asks %v", tc.swap, tc.seed, tc.paid, tl.asks)
		}
	}
}

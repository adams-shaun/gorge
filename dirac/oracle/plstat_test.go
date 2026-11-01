package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// plstat pool, whose carriers are the two `Affected$ You` statics gorge
// reads beyond Unlimited hand size and hexproof:
//
//   - a numeric maximum hand size (maxHandSizeFor): The Ten Rings (ten) and
//     Doctor Octopus, Master Planner (eight), whose end-step draws bring the
//     hand up to the maximum, so cleanup keeps (Ten Rings) or discards down
//     to it (a hand above the maximum);
//   - protection from each opponent (playerProtectedFrom): Absolute Virtue's
//     controller can't be targeted by, or dealt damage (combat or not) by,
//     an opponent's sources.

var kplDecks = [][2][]cardset.Entry{{
	// M0 Doctor Octopus (eight) vs burn and ground creatures
	{
		{Name: "Doctor Octopus, Master Planner", Count: 4}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 3},
		{Name: "Divination", Count: 4}, {Name: "Inspiration", Count: 2}, {Name: "Night's Whisper", Count: 2},
		{Name: "Opt", Count: 2}, {Name: "Wind Drake", Count: 2}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 9},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Strike", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Gigantosaurus", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	// M1 Absolute Virtue vs burn and fliers
	{
		{Name: "Absolute Virtue", Count: 4}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 3},
		{Name: "Divination", Count: 4}, {Name: "Inspiration", Count: 2}, {Name: "Opt", Count: 2},
		{Name: "Wind Drake", Count: 3}, {Name: "Island", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Wind Drake", Count: 5}, {Name: "Air Elemental", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Lightning Strike", Count: 4}, {Name: "Grizzly Bears", Count: 1}, {Name: "Flame Rift", Count: 3}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
}, {
	// M2 The Ten Rings vs burn and ground creatures
	{
		{Name: "The Ten Rings", Count: 4}, {Name: "Sol Ring", Count: 3}, {Name: "Mind Stone", Count: 4},
		{Name: "Divination", Count: 4}, {Name: "Inspiration", Count: 2}, {Name: "Opt", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Island", Count: 17},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Strike", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Gigantosaurus", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

// kplTurn is what one turn showed: its active player controlled a hand-size
// carrier, the hand size at the last decision of the turn.
type kplTurn struct {
	carrier bool
	hand    int
}

// kplStats are the coverage counters of a run.
type kplStats struct {
	games, cleanups, big  int // cleanups under a carrier; of them the hand was 8..10
	prevCombat, prevOther int // damage events prevented by protection
	virtueGames, oppBurn  int // games with Absolute Virtue played; burn cast by its controller's opponent after it entered
	maxHand               int
	stalls                map[string]int
	div                   string
}

func kplCarrier(name string) bool {
	return name == "The Ten Rings" || name == "Doctor Octopus, Master Planner"
}

// kplPolicy is eagerPolicy that also records, per turn, whether the active
// player controls a hand-size carrier and the hand size at the last decision.
func kplPolicy(seed uint64, played map[string]int, turns map[int32]*kplTurn) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			t := turns[int32(g.Turn)]
			if t == nil {
				t = &kplTurn{}
				turns[int32(g.Turn)] = t
			}
			for _, id := range g.Zone(state.ZBattlefield, g.Active) {
				if ob := g.Obj(id); ob != nil && ob.Face() != nil && kplCarrier(ob.Face().Name) {
					t.carrier = true
				}
			}
			t.hand = len(g.Zone(state.ZHand, g.Active))
		}
		return inner.Pick(c)
	})
}

// kplRun plays the seeds of every matchup in lockstep and returns the
// counters; it stops at the first divergence (st.div) when stop is set.
func kplRun(t *testing.T, seeds uint64, stop bool) *kplStats {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("plstat")
	if err != nil {
		t.Fatal(err)
	}
	st := &kplStats{stalls: map[string]int{}}
	for mi, md := range kplDecks {
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
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kpl-a", "kpl-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			played := map[string]int{}
			turns := map[int32]*kplTurn{}
			inner := kplPolicy(seed, played, turns)
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			st.games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls[res.Stall]++
			}
			for _, tn := range turns {
				if tn.carrier {
					st.cleanups++
					if tn.hand >= 8 && tn.hand <= 10 {
						st.big++
					}
					st.maxHand = max(st.maxHand, tn.hand)
				}
			}
			kplScan(gs, st)
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				st.div = strings.Join(append([]string{
					fmt.Sprintf("matchup %d seed %d", mi, seed), d.What, d.Field, fmt.Sprintf("at %d", d.At),
				}, actionStrings(res.Actions[lo:d.At])...), " | ")
				if stop {
					return st
				}
			}
		}
	}
	return st
}

// kplScan counts, in gorge's event log, the damage events protection
// prevented (combat or not) and the burn an Absolute Virtue controller's
// opponent cast after it entered.
func kplScan(g *GorgeSide, st *kplStats) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	step := state.StepUntap
	virtue := false
	var virtueCtrl state.PlayerID
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.StepChange:
			step = ev.Step
		case events.MoveZone:
			if ev.To == state.ZBattlefield && name(ev.Obj) == "Absolute Virtue" {
				if !virtue {
					st.virtueGames++
				}
				virtue = true
				if ob := g.E.G.Obj(ev.Obj); ob != nil {
					virtueCtrl = ob.Controller
				}
			}
		case events.Note:
			if ev.Text == "prevented: protection" && ev.Obj == 0 {
				if step == state.StepCombatDamage {
					st.prevCombat++
				} else {
					st.prevOther++
				}
			}
		case events.PutOnStack:
			if n := name(ev.Obj); virtue && ev.Player != virtueCtrl && (n == "Shock" || n == "Lightning Strike") {
				st.oppBurn++
			}
		}
	}
}

func TestPlstatLockstep(t *testing.T) {
	t.Parallel()

	st := kplRun(t, 256, true)
	t.Logf("%d games; kernel stalls %v; cleanups with a carrier %d, hand 8..10 under a carrier %d (largest %d); "+
		"damage prevented by protection: combat %d, other %d; Absolute Virtue games %d, opponent burn cast after it entered %d",
		st.games, st.stalls, st.cleanups, st.big, st.maxHand, st.prevCombat, st.prevOther, st.virtueGames, st.oppBurn)
	if st.div != "" {
		t.Fatalf("diverged: %s", st.div)
	}
	if len(st.stalls) != 0 {
		t.Errorf("kernel stalls %v", st.stalls)
	}
	if st.cleanups < 10 {
		t.Errorf("cleanups with a carrier %d, want at least 10", st.cleanups)
	}
	if st.big < 5 {
		t.Errorf("cleanups with a hand of 8..10 under a carrier %d, want at least 5", st.big)
	}
	if st.prevCombat+st.prevOther < 10 || st.prevCombat < 1 || st.prevOther < 1 {
		t.Errorf("damage prevented by protection: combat %d, other %d, want at least 10 with both kinds", st.prevCombat, st.prevOther)
	}
}

// TestPlstatMutation: with the numeric maximum hand size ignored
// (dirac.DebugIgnoreHandSize) Doctor Octopus's end-step draw to eight is
// discarded to seven in the kernel only, so the lockstep diverges.
func TestPlstatMutation(t *testing.T) {
	dirac.DebugIgnoreHandSize = true
	defer func() { dirac.DebugIgnoreHandSize = false }()

	st := kplRun(t, 256, true)
	if st.div == "" {
		t.Fatalf("no divergence in %d games with the maximum hand size ignored", st.games)
	}
	t.Logf("diverged as wanted: %s", st.div)
}

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Void lockstep: Count$Void.<yes>.<no>, Edge of Eternities'
// "a nonland permanent left the battlefield this turn or a spell was warped
// this turn" (gorge cov3BranchHoldsVoid). Deck A holds eight carriers plus
// burn and bears (the permanents that leave); deck B holds the other three
// carriers, three warp creatures, bears and Evolving Wilds (a LAND that
// leaves, the half the mutation breaks: DebugVoidLandToo).
//
// The counts come from gorge's state and event log, once per (game, turn,
// class) at the first decision where the class holds: a Void that is true only
// through a nonland permanent leaving, only through a warp cast, through
// both, false, and true by an opponent's permanent or warp (opponent: not the
// active player, who controls every sorcery and end-step carrier).
var kvoidDecks = [][2][]cardset.Entry{{
	{
		{Name: "Plasma Bolt", Count: 2}, {Name: "Tragic Trajectory", Count: 2}, {Name: "Hymn of the Faller", Count: 2},
		{Name: "Decode Transmissions", Count: 2}, {Name: "Elegy Acolyte", Count: 2}, {Name: "Voidforged Titan", Count: 2},
		{Name: "Insatiable Skittermaw", Count: 2}, {Name: "Kavaron Skywarden", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 1},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 2}, {Name: "Evolving Wilds", Count: 2},
	},
	{
		{Name: "Hylderblade", Count: 3}, {Name: "Interceptor Mechan", Count: 3}, {Name: "Temporal Intervention", Count: 3},
		{Name: "Germinating Wurm", Count: 3}, {Name: "Mightform Harmonizer", Count: 3}, {Name: "Nova Hellkite", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Evolving Wilds", Count: 5},
	},
}}

// kvoidTally classifies gorge's Void at each decision.
type kvoidTally struct {
	game        int
	seen        map[[3]int32]bool // (game, turn, class)
	leaveOnly   int
	warpOnly    int
	both        int
	falseReads  int
	opp         int
	warpCasts   int
	cursorEv    int
	cursorTurn  int32
	curLeave    bool
	curLeaveOpp bool
	curWarp     bool
	curWarpOpp  bool
}

func (w *kvoidTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	leave, leaveOpp := false, false
	for _, en := range g.Entered {
		if en.From != state.ZBattlefield {
			continue
		}
		o := g.Obj(en.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		if o.Face().IsLand() {
			continue
		}
		leave = true
		if o.Owner != g.Active {
			leaveOpp = true
		}
	}
	// the warp casts of this turn: a PutOnStack, then its CastInfo carrying
	// FlagWarped (the pay-time flags sit below the push)
	warp, warpOpp := false, false
	start := 0
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == events.TurnChange {
			start = i
			break
		}
	}
	pusher := map[state.ObjID]state.PlayerID{}
	for _, e := range evs[start:] {
		switch e.Kind {
		case events.PutOnStack:
			pusher[e.Obj] = e.Player
		case events.CastInfo:
			if events.FlagsFrom(e.Counter)&state.FlagWarped != 0 {
				warp = true
				if p, ok := pusher[e.Obj]; ok && p != g.Active {
					warpOpp = true
				}
			}
		}
	}
	class := int32(0)
	switch {
	case leave && warp:
		class = 3
	case leave:
		class = 1
	case warp:
		class = 2
	}
	key := [3]int32{int32(w.game), g.Turn, class}
	if w.seen[key] {
		return nil
	}
	w.seen[key] = true
	switch class {
	case 0:
		w.falseReads++
	case 1:
		w.leaveOnly++
	case 2:
		w.warpOnly++
	case 3:
		w.both++
	}
	if leaveOpp || warpOpp {
		w.opp++
	}
	return nil
}

// kvoidPolicy casts the warp cost whenever it is offered, else plays or casts
// at random (so Evolving Wilds cracks and creatures die).
func kvoidPolicy(seed uint64, w *kvoidTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && a.Mode()&0x0f == 8 { // canonical "warped"
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			w.warpCasts++
		}
		return a
	})
}

func TestVoidLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("void")
	if err != nil {
		t.Fatal(err)
	}
	w := &kvoidTally{seen: map[[3]int32]bool{}}
	games, divs, stalls := 0, 0, 0
	for mi, md := range kvoidDecks {
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
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kvoid-a", "kvoid-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := kvoidPolicy(seed, w)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			w.game = games
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: w.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				divs++
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Errorf("matchup %d seed %d: kernel capacity stall %q", mi, seed, res.Stall)
			}
		}
	}
	t.Logf("%d games, %d divergences, %d stalls; Void classes (per game-turn): %d leave-only, %d warp-only, %d both, %d false, %d by an opponent's permanent or warp; %d policy warp picks",
		games, divs, stalls, w.leaveOnly, w.warpOnly, w.both, w.falseReads, w.opp, w.warpCasts)
	if w.leaveOnly < 20 {
		t.Errorf("%d Void turns true only through a nonland permanent leaving, want >= 20", w.leaveOnly)
	}
	if w.warpOnly < 10 {
		t.Errorf("%d Void turns true only through a warp cast, want >= 10", w.warpOnly)
	}
	if w.falseReads < 20 {
		t.Errorf("%d false Void turns, want >= 20", w.falseReads)
	}
	if w.opp < 5 {
		t.Errorf("%d Void turns true by an opponent's permanent or warp, want >= 5", w.opp)
	}
}

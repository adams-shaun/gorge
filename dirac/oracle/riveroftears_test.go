package oracle

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The River of Tears lockstep: "{T}: Add {U}. If you played a land this turn,
// add {B} instead" is a Condition-split mana source (kop.Ability.SplitGate;
// dirac/mana_condsplit.go). gorge's planner defers it (pay source:conditional),
// so only a potential play's prefix search or the mana window taps it, and its
// relaxed census unit is {U}, or {U} or {B} once a land is played. The pool has
// no other blue or black source: a blue one-drop (Faerie Seer, Opt) is payable
// only by River before the turn's land drop, a black one (Carrion Feeder,
// Disfigure) only after it. The expected answer at every decision is gorge's,
// on both surfaces.
var kriverDecks = [][2][]cardset.Entry{{
	{
		{Name: "River of Tears", Count: 8}, {Name: "Faerie Seer", Count: 6}, {Name: "Carrion Feeder", Count: 6},
		{Name: "Opt", Count: 4}, {Name: "Disfigure", Count: 4}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 14}, {Name: "Mountain", Count: 14},
	},
}, {
	{
		{Name: "River of Tears", Count: 8}, {Name: "Faerie Seer", Count: 6}, {Name: "Carrion Feeder", Count: 6},
		{Name: "Opt", Count: 4}, {Name: "Disfigure", Count: 4}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "River of Tears", Count: 10}, {Name: "Faerie Seer", Count: 4}, {Name: "Carrion Feeder", Count: 4},
		{Name: "Opt", Count: 4}, {Name: "Disfigure", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 12},
	},
}}

// kriverTally counts, on gorge's side, what River of Tears made: blueBefore a
// {U} tap before the controller's land drop of the turn, blackAfter a {B} tap
// after it (each arm of the split), and wrong the taps that made the other
// colour (the live gate disagreeing with the tally's own reading).
type kriverTally struct {
	games, blueBefore, blackAfter, wrong int
}

func kriverRun(t *testing.T, first, last uint64, report bool, tal *kriverTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("riveroftears")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kriverDecks {
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
		for seed := first; seed <= last; seed++ {
			if v, err := strconv.ParseUint(os.Getenv("KRIVER_ONLY"), 10, 64); err == nil && seed != v {
				continue
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"rt-a", "rt-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			var landed [2]bool          // each seat has played a land this turn
			pend := state.PlayerID(255) // the seat whose River tap awaits its mana, 255 none
			var pendLanded bool
			var refSide *GorgeSide
			check := func(ref *GorgeSide, _ Side) *Divergence {
				refSide = ref
				evs := ref.E.L.Events
				for ; seen < len(evs) && tal != nil; seen++ {
					ev := evs[seen]
					switch ev.Kind {
					case events.TurnChange:
						// gorge resets a seat's LandsPlayed only when its own turn
						// begins (events foldTurnChange), so on an opponent's turn
						// the gate still reads the seat's last own turn
						if int(ev.Player) < 2 {
							landed[ev.Player] = false
						}
					case events.LandPlayed:
						if int(ev.Player) < 2 {
							landed[ev.Player] = true
						}
					case events.Tap:
						if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "River of Tears" {
							pend, pendLanded = o.Controller, landed[o.Controller]
						}
					case events.ManaAdd:
						if pend != 255 && ev.Player == pend && ev.Amount > 0 {
							switch {
							case !pendLanded && ev.Counter == "U":
								tal.blueBefore++
							case pendLanded && ev.Counter == "B":
								tal.blackAfter++
							default:
								tal.wrong++
							}
							pend = 255
						}
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20, Check: check})
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
					return div
				}
				if os.Getenv("KRIVER_DEBUG") != "" && refSide != nil {
					g := refSide.E.G
					for p := range g.Players {
						t.Logf("p%d lands played %d pool %v", p, g.Players[p].LandsPlayed, g.Players[p].Pool)
						for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
							if o := g.Obj(id); o != nil && o.Face() != nil {
								t.Logf("  bf #%d %s tapped=%v", id, o.Face().Name, o.Tapped)
							}
						}
						for _, id := range g.Zone(state.ZHand, state.PlayerID(p)) {
							if o := g.Obj(id); o != nil && o.Face() != nil {
								t.Logf("  hand #%d %s", id, o.Face().Name)
							}
						}
					}
					t.Logf("turn %d step %v active %v", g.Turn, g.Step, g.Active)
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

// TestRiverOfTearsDeferredManaLockstep plays seeds 1..300 of both matchups, on both
// surfaces (odd seeds the full one), as four parallel chunks of 75 seeds that
// share the fixtures: each must stay in lockstep and reach both arms of the split.
func TestRiverOfTearsDeferredManaLockstep(t *testing.T) {
	t.Parallel()
	const chunk = 75
	for first := uint64(1); first <= 300; first += chunk {
		t.Run(fmt.Sprintf("seeds%03d-%03d", first, first+chunk-1), func(t *testing.T) {
			t.Parallel()
			var tal kriverTally
			if n := kriverRun(t, first, first+chunk-1, true, &tal); n != 0 {
				t.Fatalf("%d seeds diverged", n)
			}
			t.Logf("%d games: %+v", tal.games, tal)
			// both arms of the split are reached, and every tap made what its board says
			if tal.blueBefore < 10 || tal.blackAfter < 10 || tal.wrong != 0 {
				t.Errorf("arm coverage too thin or River made the wrong colour: %+v", tal)
			}
		})
	}
}

// TestRiverOfTearsLiveGateMutation: with the arm's live gate forced off some
// seed of the first 40 diverges from gorge, proof the pool exercises it.
func TestRiverOfTearsLiveGateMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCondSplitGate = true
	defer func() { dirac.DebugNoCondSplitGate = false }()
	if kriverRun(t, 1, 40, false, nil) == 0 {
		t.Fatal("no seed diverged with the Condition-split arm's gate forced off: the pool does not exercise it")
	}
}

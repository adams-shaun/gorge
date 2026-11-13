package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CantBeCast NumLimitEachTurn$ and granted view-only look lockstep
// High Noon ("Each player can't cast more than one
// spell each turn.") beside Glowcap Lantern (an Equipment granting a
// library-top look and an attack trigger). The kernel's histSpells cap must
// offer exactly the casts gorge's castLimitBinds does.
var kstp2Decks = [2][]cardset.Entry{
	{
		{Name: "Glowcap Lantern", Count: 4}, {Name: "High Noon", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Savannah Lions", Count: 4}, {Name: "Giant Spider", Count: 3}, {Name: "Forest", Count: 10},
		{Name: "Plains", Count: 9},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Divination", Count: 4},
		{Name: "Hill Giant", Count: 6}, {Name: "Gray Ogre", Count: 4}, {Name: "Mountain", Count: 9},
		{Name: "Island", Count: 9},
	},
}

type kstp2Stats struct {
	games, divs, stalls int
	noonTurns           int // turns with a High Noon on the battlefield in which a spell was put on the stack
	lanternAttacks      int // attacks by a creature with a Glowcap Lantern attached
	violations          int // gorge let a player put a second spell on the stack with a High Noon out
}

func runCastlimitlook(t *testing.T, seeds int, off bool) kstp2Stats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("castlimitlook")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCastLimitOff
	dirac.DebugCastLimitOff = off
	t.Cleanup(func() { dirac.DebugCastLimitOff = old })
	var decks [2][]*cards.Card
	for s, d := range kstp2Decks {
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
	tbl, _, _, err := set.Matchup(kstp2Decks[0], kstp2Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st kstp2Stats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kstp2-a", "kstp2-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		var cast [2]int // spells put on the stack this turn, per player
		noonTurn := false
		prevNoon := false
		check := func(ref *GorgeSide, _ Side) *Divergence {
			g := ref.E.G
			noon := false
			for p := 0; p < 2; p++ {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "High Noon" {
						noon = true
					}
				}
			}
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				switch ev.Kind {
				case events.TurnChange:
					cast, noonTurn = [2]int{}, false
				case events.PutOnStack:
					if o := g.Obj(ev.Obj); o == nil || o.Ability != nil || int(ev.Player) > 1 {
						continue
					}
					cast[ev.Player]++
					if noon || prevNoon {
						if !noonTurn {
							st.noonTurns++
							noonTurn = true
						}
						if cast[ev.Player] >= 2 && noon && prevNoon {
							st.violations++
						}
					}
				case events.DeclareAttackers:
					for _, id := range ev.IDs {
						for p := 0; p < 2; p++ {
							for _, eq := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
								if o := g.Obj(eq); o != nil && o.AttachedTo == id && o.Face() != nil && o.Face().Name == "Glowcap Lantern" {
									st.lanternAttacks++
								}
							}
						}
					}
				}
			}
			prevNoon = noon
			return nil
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if !off {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if res.Stall != "" && res.Stall != "turns" {
			st.stalls++
			if !off {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return st
}

func TestCastLimitAndGrantedLookLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCastlimitlook(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d High Noon turns with a spell, %d Lantern attacks, %d gorge double casts", st.games, st.divs, st.stalls, st.noonTurns, st.lanternAttacks, st.violations)
	if st.violations != 0 {
		t.Fatalf("gorge let a player put a second spell on the stack with a High Noon out (%d times)", st.violations)
	}
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.noonTurns < 30 {
		t.Errorf("only %d High Noon turns with a spell, want >= 30", st.noonTurns)
	}
	if st.lanternAttacks < 20 {
		t.Errorf("only %d Glowcap Lantern attacks, want >= 20", st.lanternAttacks)
	}
}

// TestCastLimitAndGrantedLookMutation ignores the cap: the kernel then forbids every cast
// while a High Noon is out, which gorge does not, so some game must diverge.
func TestCastLimitAndGrantedLookMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCastlimitlook(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left the cap off in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}

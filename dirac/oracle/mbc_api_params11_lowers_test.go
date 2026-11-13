package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCApiParams11Lowers plays the MBC api-params-11 spellings kgen lowers in
// lockstep with gorge over the mbcapiparams11 fixture pool
// (dirac/cardset/pools/fixtures/mbcapiparams11):
//
//   - PresentPlayer: a trigger's IsPresent$ ... | PresentZone$ Graveyard |
//     PresentPlayer$ You (Pit Keeper's ETB, GE4; Gixian Skullflayer's upkeep,
//     GE3) counts only the controller's graveyard (gorge presentClauseHolds,
//     kgen presentPlayerCond). The black deck faces a green creature deck, so
//     both graveyards fill; the precondition is that Pit Keeper entered both
//     with four creature cards in its controller's graveyard and with fewer
//     there but four or more across both graveyards (the entries an unscoped
//     count would fire on), and that the trigger was put on the stack.
//   - EachDamage: Nissa's Judgment's DB$ EachDamage | ValidCards$
//     Creature.YouCtrl+counters_GE1_P1P1 is the DefinedDamagers$ Valid sweep
//     (gorge effEachDamage); the precondition is that it resolved and dealt
//     damage.
//
// Each mutation subtest drops the lowered piece from the table (the
// intervening-if conditions; the EachDamage instruction) and must diverge.
// Ram Through's ExcessDamage$ and Necrologia's PlayerTurn$ stay refused: see
// the report.
func TestMBCApiParams11Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runAP11Lockstep(t, 24, true, nil)
		t.Logf("%d games, %d divergences, %d stalls; Pit Keeper entries %+v, Nissa's Judgment %+v", run.games, run.divs, run.stalls, st.keeper, st.nissa)
		if st.keeper.own < 2 {
			t.Errorf("Pit Keeper entered with four creature cards in its controller's graveyard %d times, want at least 2", st.keeper.own)
		}
		if st.keeper.scoped < 2 {
			t.Errorf("Pit Keeper entered with fewer than four in its controller's graveyard but four or more in all %d times, want at least 2", st.keeper.scoped)
		}
		if st.keeper.pushed < 2 {
			t.Errorf("Pit Keeper's trigger went on the stack %d times, want at least 2", st.keeper.pushed)
		}
		if st.nissa.dealt < 3 {
			t.Errorf("Nissa's Judgment resolved and dealt damage %d times, want at least 3", st.nissa.dealt)
		}
	})
	t.Run("mutation-PresentPlayer", func(t *testing.T) {
		run, _ := runAP11Lockstep(t, 24, false, func(tbl *kop.Table) {
			for _, name := range [...]string{"Pit Keeper", "Gixian Skullflayer"} {
				mutateCard(t, tbl, name, func(a *kop.Ability, _ *kop.Instr) {
					if a.Kind == kop.AbTriggered {
						a.Trig.Cond = 0
					}
				})
			}
		})
		run.requireDivergence(t, "a graveyard-count intervening-if dropped")
	})
	t.Run("mutation-EachDamage", func(t *testing.T) {
		run, _ := runAP11Lockstep(t, 24, false, func(tbl *kop.Table) {
			mutateCard(t, tbl, "Nissa's Judgment", func(_ *kop.Ability, in *kop.Instr) {
				if in.Ext() == kop.ExtEachDamage {
					*in = kop.Instr{Op: kop.OpNop}
				}
			})
		})
		run.requireDivergence(t, "Nissa's Judgment without its EachDamage")
	})
}

var ap11Decks = fixtureDecks{
	{{Name: "Pit Keeper", Count: 8}, {Name: "Gixian Skullflayer", Count: 4}, {Name: "Walking Corpse", Count: 6}, {Name: "Typhoid Rats", Count: 6}, {Name: "Swamp", Count: 16}},
	{{Name: "Nissa's Judgment", Count: 8}, {Name: "Grizzly Bears", Count: 14}, {Name: "Forest", Count: 18}},
}

var ap11Watch = map[string]bool{"Pit Keeper": true, "Gixian Skullflayer": true, "Nissa's Judgment": true}

// ap11Stats are the preconditions measured from gorge's side.
type ap11Stats struct {
	keeper struct {
		own, scoped, under, pushed int // entries with >= 4 in its controller's graveyard; < 4 there, >= 4 in all; < 4 in all; triggers pushed
	}
	nissa struct {
		resolved, dealt int // resolutions; resolutions that dealt damage to a permanent
	}
}

// creatureCards counts the creature cards in p's graveyard.
func creatureCards(g *state.Game, p state.PlayerID) int {
	n := 0
	for _, id := range g.Zone(state.ZGraveyard, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().IsCreature() {
			n++
		}
	}
	return n
}

// ap11Observe scans the events gorge appended since *seen.
func ap11Observe(ref *GorgeSide, seen *int, nissaOn *state.ObjID, nissaHit *bool, st *ap11Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; *seen < len(evs); *seen++ {
		e := evs[*seen]
		o := g.Obj(e.Obj)
		name := ""
		if o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && name == "Pit Keeper":
			own, all := creatureCards(g, o.Controller), 0
			for p := range g.Players {
				all += creatureCards(g, state.PlayerID(p))
			}
			switch {
			case own >= 4:
				st.keeper.own++
			case all >= 4:
				st.keeper.scoped++
			default:
				st.keeper.under++
			}
		case e.Kind == events.TriggerPush && name == "Pit Keeper":
			st.keeper.pushed++
		case e.Kind == events.Resolve && e.Text == "Nissa's Judgment":
			*nissaOn, *nissaHit = e.Obj, false
			st.nissa.resolved++
		case e.Kind == events.Damage && *nissaOn != 0 && e.Obj != 0 && e.Amount > 0:
			*nissaHit = true
		case e.Kind == events.MoveZone && e.Obj == *nissaOn && e.From == state.ZStack:
			if *nissaHit {
				st.nissa.dealt++
			}
			*nissaOn = 0
		}
	}
}

// runAP11Lockstep plays seeds 1..seeds of the mbcapiparams11 matchup, gorge
// leading, measuring the preconditions; mutate rewrites the table first.
func runAP11Lockstep(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) (fixtureRun, ap11Stats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcapiparams11")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ap11Decks {
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
	tbl, _, _, err := set.Matchup(ap11Decks[0], ap11Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	run := fixtureRun{casts: map[string]int{}}
	var st ap11Stats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ap11-a", "ap11-b"}, decks, 1)
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
		seen, nissaOn, nissaHit := 0, state.ObjID(0), false
		check := func(ref *GorgeSide, _ Side) *Divergence {
			ap11Observe(ref, &seen, &nissaOn, &nissaHit, &st)
			return nil
		}
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, ap11Watch, run.casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
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

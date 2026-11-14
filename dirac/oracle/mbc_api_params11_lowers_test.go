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
//     count would fire on), and that the trigger was put on the stack. The
//     second matchup plays the Battlefield,Graveyard sum (Sand Strangler: a
//     Desert you control or a Desert card in your graveyard, both decks
//     cycling Deserts) and the Battlefield-only form (Eiganjo Free-Riders);
//     the precondition is Sand Strangler entering both with a Desert of its
//     controller's and with Deserts only on the other side.
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
		t.Logf("%d games, %d divergences, %d stalls; Pit Keeper entries %+v, Nissa's Judgment %+v, Sand Strangler entries %+v", run.games, run.divs, run.stalls, st.keeper, st.nissa, st.strangler)
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
		if st.strangler.own < 2 || st.strangler.scoped < 2 || st.strangler.pushed < 2 {
			t.Errorf("Sand Strangler entries %+v: want at least 2 with a Desert of its controller's, 2 with Deserts only on the other side, and 2 triggers pushed", st.strangler)
		}
	})
	t.Run("mutation-PresentPlayer", func(t *testing.T) {
		run, _ := runAP11Lockstep(t, 24, false, func(tbl *kop.Table, mi int) {
			names := []string{"Pit Keeper", "Gixian Skullflayer"}
			if mi == 1 {
				names = []string{"Sand Strangler"}
			}
			for _, name := range names {
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
		run, _ := runAP11Lockstep(t, 24, false, func(tbl *kop.Table, mi int) {
			if mi != 0 {
				return
			}
			mutateCard(t, tbl, "Nissa's Judgment", func(_ *kop.Ability, in *kop.Instr) {
				if in.Ext() == kop.ExtEachDamage {
					*in = kop.Instr{Op: kop.OpNop}
				}
			})
		})
		run.requireDivergence(t, "Nissa's Judgment without its EachDamage")
	})
}

var ap11Decks = []fixtureDecks{{
	{{Name: "Pit Keeper", Count: 8}, {Name: "Gixian Skullflayer", Count: 4}, {Name: "Walking Corpse", Count: 6}, {Name: "Typhoid Rats", Count: 6}, {Name: "Swamp", Count: 16}},
	{{Name: "Nissa's Judgment", Count: 8}, {Name: "Grizzly Bears", Count: 14}, {Name: "Forest", Count: 18}},
}, {
	{{Name: "Sand Strangler", Count: 10}, {Name: "Raging Goblin", Count: 12}, {Name: "Desert of the Fervent", Count: 5}, {Name: "Mountain", Count: 13}},
	{{Name: "Eiganjo Free-Riders", Count: 8}, {Name: "Savannah Lions", Count: 14}, {Name: "Desert of the True", Count: 5}, {Name: "Plains", Count: 13}},
}}

var ap11Watch = map[string]bool{"Pit Keeper": true, "Gixian Skullflayer": true, "Nissa's Judgment": true, "Sand Strangler": true, "Eiganjo Free-Riders": true}

// ap11Stats are the preconditions measured from gorge's side.
type ap11Stats struct {
	keeper struct {
		own, scoped, under, pushed int // entries with >= 4 in its controller's graveyard; < 4 there, >= 4 in all; < 4 in all; triggers pushed
	}
	strangler struct {
		own, scoped, none, pushed int // entries with a Desert of its controller's; Deserts only on the other side; none anywhere; triggers pushed
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

// deserts counts the Desert permanents p controls plus the Desert cards in
// p's graveyard (Sand Strangler's PresentZone$ Battlefield,Graveyard read).
func deserts(g *state.Game, p state.PlayerID) int {
	n := 0
	desert := func(o *state.Object) bool { return o != nil && o.Face() != nil && o.Face().HasTypeWord("Desert") }
	for _, q := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, q) {
			if o := g.Obj(id); desert(o) && o.Controller == p {
				n++
			}
		}
	}
	for _, id := range g.Zone(state.ZGraveyard, p) {
		if desert(g.Obj(id)) {
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
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && name == "Sand Strangler":
			own, all := deserts(g, o.Controller), 0
			for p := range g.Players {
				all += deserts(g, state.PlayerID(p))
			}
			switch {
			case own > 0:
				st.strangler.own++
			case all > 0:
				st.strangler.scoped++
			default:
				st.strangler.none++
			}
		case e.Kind == events.TriggerPush && name == "Sand Strangler":
			st.strangler.pushed++
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

// runAP11Lockstep plays seeds 1..seeds of the mbcapiparams11 matchups, gorge
// leading, alternating the matchup and the option surface by seed and
// measuring the preconditions; mutate rewrites each matchup's table first.
func runAP11Lockstep(t *testing.T, seeds int, report bool, mutate func(tbl *kop.Table, matchup int)) (fixtureRun, ap11Stats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcapiparams11")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(ap11Decks))
	tbls := make([]*kop.Table, len(ap11Decks))
	for mi, md := range ap11Decks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for range e.Count {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl, mi)
		}
		tbls[mi] = tbl
	}
	run := fixtureRun{casts: map[string]int{}}
	var st ap11Stats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(ap11Decks)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ap11-a", "ap11-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
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

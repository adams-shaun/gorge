package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC Player-head filters kgen lowers where the trigger reads them, each
// card played against gorge in a fixture-table lockstep game
// (printedDelveSet). O-Kagachi, Vengeful Kami's "deals combat damage to a
// player, if that player attacked you during their last turn" is ValidTarget$
// Player.attackedYouTheirLastTurn: the kernel's per-seat attack record
// (kop.Tf5DmgAttackedYouLastTurn, dirac/trig_atklastturn.go) against gorge's
// state.Player.AttackedSeatsLastTurn. The opponent's random attacks make some
// of O-Kagachi's attacks punish and some not; the mutation drops the record's
// gate, so a hit on a player who did not attack fires the exile, and must
// diverge.
var okagachiDecks = [2][]cardset.Entry{
	{
		{Name: "O-Kagachi, Vengeful Kami", Count: 8}, {Name: "Birds of Paradise", Count: 4}, {Name: "Manalith", Count: 3},
		{Name: "City of Brass", Count: 4}, {Name: "Mana Confluence", Count: 4}, {Name: "Gemstone Mine", Count: 2},
		{Name: "Plains", Count: 3}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Raging Goblin", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Manalith", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}

// playerHeadTally counts, from gorge's events, O-Kagachi's attacks on a player
// who attacked its controller during their last turn (punish) and on one who
// did not (spare), and its trigger's pushes (fired).
type playerHeadTally struct {
	punish, spare, fired int
}

func okagachiCheck(tal *playerHeadTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
		}
		isOKa := func(id state.ObjID) bool {
			o := ref.E.G.Obj(id)
			return o != nil && o.Face() != nil && o.Face().Name == "O-Kagachi, Vengeful Kami"
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			switch ev := evs[seen]; ev.Kind {
			case events.DeclareAttackers:
				for _, id := range ev.IDs {
					if !isOKa(id) {
						continue
					}
					if ref.E.AttackedDuringLastTurn(ev.Player, ref.E.G.Obj(id).Controller) {
						tal.punish++
					} else {
						tal.spare++
					}
				}
			case events.TriggerPush:
				if isOKa(ev.Obj) {
					tal.fired++
				}
			}
		}
		return nil
	}
}

// okagachiRun plays n seeds of okagachiDecks and returns the divergences.
func okagachiRun(t *testing.T, n uint64, tal *playerHeadTally, mut func(*kop.Table)) int {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	var names [2][]string
	for s, d := range okagachiDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(okagachiDecks[0], okagachiDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	div := 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"okagachi", "aggro"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 6000, MaxTurns: 40, Check: okagachiCheck(tal)})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if mut != nil {
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

func TestMBCPlayerHeadLowers(t *testing.T) {
	t.Parallel()
	t.Run("O-Kagachi, Vengeful Kami", func(t *testing.T) {
		t.Parallel()
		var tal playerHeadTally
		okagachiRun(t, 120, &tal, nil)
		t.Logf("O-Kagachi: %d attacks on a player who attacked last turn, %d on one who did not, %d trigger pushes", tal.punish, tal.spare, tal.fired)
		if tal.punish < 3 || tal.spare < 3 || tal.fired < 3 {
			t.Errorf("want at least 3 of each: punish %d, spare %d, fired %d", tal.punish, tal.spare, tal.fired)
		}
	})
	t.Run("O-Kagachi mutation", func(t *testing.T) {
		t.Parallel()
		var tal playerHeadTally
		n := okagachiRun(t, 120, &tal, func(tbl *kop.Table) {
			dropped := 0
			cds := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cds {
				abs := append([]kop.Ability(nil), cds[i].Abilities...)
				for j := range abs {
					if abs[j].Trig.Flags5&kop.Tf5DmgAttackedYouLastTurn != 0 {
						abs[j].Trig.Flags5 &^= kop.Tf5DmgAttackedYouLastTurn
						dropped++
					}
				}
				cds[i].Abilities = abs
			}
			if dropped == 0 {
				t.Fatal("precondition: no trigger carries Tf5DmgAttackedYouLastTurn in the table")
			}
			tbl.Cards = cds
		})
		if n == 0 {
			t.Fatalf("no seed diverged with the attack-record gate dropped (spare %d)", tal.spare)
		}
	})
	for _, oath := range []string{"Oath of Druids", "Oath of Scholars", "Oath of Mages"} {
		t.Run(oath, func(t *testing.T) {
			t.Parallel()
			var tal oathTally
			oathRun(t, oath, 120, &tal, nil)
			t.Logf("%s: %d triggers pushed, %d resolved, %d creatures dug onto the battlefield", oath, tal.pushed, tal.resolved, tal.dug)
			if tal.resolved < 3 || tal.pushed-tal.resolved < 3 || oath == "Oath of Druids" && tal.dug < 3 {
				t.Errorf("want at least 3 resolved and 3 removed (3 dug for Druids): pushed %d, resolved %d, dug %d", tal.pushed, tal.resolved, tal.dug)
			}
		})
		t.Run(oath+" mutation", func(t *testing.T) {
			t.Parallel()
			var tal oathTally
			dropped := 0
			n := oathRun(t, oath, 120, &tal, func(tbl *kop.Table) { dropped = dropOathTargetReading(tbl) })
			if dropped == 0 {
				t.Fatal("precondition: no target slot carries the active-player reading in the table")
			}
			if n == 0 {
				t.Fatalf("no seed diverged with the active-player target reading dropped (pushed %d, resolved %d)", tal.pushed, tal.resolved)
			}
		})
	}
}

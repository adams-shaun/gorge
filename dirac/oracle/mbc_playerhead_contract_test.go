package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Miss Highwater's "deals combat damage to a player who doesn't have a
// contract counter" is ValidTarget$ Player.counters_EQ0_Contract: the
// kernel's per-seat contract count (kop.Tf5DmgNoContract,
// dirac/ctr_contract.go) against gorge's state.Player.Counter("Contract").
// The damaged player may discard their hand (the switched unless cost); if
// they do they draw seven and get a contract counter, so a later hit on them
// must not fire. The mutation drops the gate, so a hit on a contracted
// player fires, and must diverge.
var highwaterDecks = [2][]cardset.Entry{
	{
		{Name: "Miss Highwater", Count: 10}, {Name: "Dark Ritual", Count: 4}, {Name: "Swamp", Count: 16},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Raging Goblin", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Manalith", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}

// contractTally counts, from gorge's events, Miss Highwater's combat hits on
// a player with no contract counter (fresh) and on one with one (spare), its
// trigger's pushes (fired) and the contract counters put (paid).
type contractTally struct {
	fresh, spare, fired, paid int
}

func highwaterCheck(tal *contractTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	var contract [MaxSeats]int
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, contract = ref, 0, [MaxSeats]int{}
		}
		isMH := func(id state.ObjID) bool {
			o := ref.E.G.Obj(id)
			return o != nil && o.Face() != nil && o.Face().Name == "Miss Highwater"
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			switch ev := evs[seen]; ev.Kind {
			case events.DamageProvenance:
				if !isMH(ev.Obj) || len(ev.IDs) != 1 || ref.E.G.Step != state.StepCombatDamage {
					continue
				}
				if p, ok := ev.IDs[0].PlayerRef(); ok && int(p) < MaxSeats {
					if contract[p] == 0 {
						tal.fresh++
					} else {
						tal.spare++
					}
				}
			case events.PlayerCounterChange:
				if ev.Counter == "Contract" && int(ev.Player) < MaxSeats {
					contract[ev.Player] += int(ev.Amount)
					tal.paid++
				}
			case events.TriggerPush:
				if isMH(ev.Obj) {
					tal.fired++
				}
			}
		}
		return nil
	}
}

// highwaterRun plays n seeds of highwaterDecks and returns the divergences.
func highwaterRun(t *testing.T, n uint64, tal *contractTally, mut func(*kop.Table)) int {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	var names [2][]string
	for s, d := range highwaterDecks {
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
	tbl, _, _, err := set.Matchup(highwaterDecks[0], highwaterDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	div := 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"highwater", "aggro"}, gd, 1)
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
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 6000, MaxTurns: 40, Check: highwaterCheck(tal)})
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

func highwaterLowers(t *testing.T) {
	t.Parallel()
	var tal contractTally
	highwaterRun(t, 120, &tal, nil)
	t.Logf("Miss Highwater: %d hits on a player with no contract counter, %d on one with one, %d trigger pushes, %d contract counters put",
		tal.fresh, tal.spare, tal.fired, tal.paid)
	if tal.fresh < 3 || tal.spare < 3 || tal.fired < 3 || tal.paid < 3 {
		t.Errorf("want at least 3 of each: fresh %d, spare %d, fired %d, paid %d", tal.fresh, tal.spare, tal.fired, tal.paid)
	}
	if tal.fired != tal.fresh {
		t.Errorf("gorge pushed %d triggers for %d hits on a player with no contract counter", tal.fired, tal.fresh)
	}
}

func highwaterMutation(t *testing.T) {
	t.Parallel()
	var tal contractTally
	n := highwaterRun(t, 120, &tal, func(tbl *kop.Table) {
		dropped := 0
		cds := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cds {
			abs := append([]kop.Ability(nil), cds[i].Abilities...)
			for j := range abs {
				if abs[j].Trig.Flags5&kop.Tf5DmgNoContract != 0 {
					abs[j].Trig.Flags5 &^= kop.Tf5DmgNoContract
					dropped++
				}
			}
			cds[i].Abilities = abs
		}
		if dropped == 0 {
			t.Fatal("precondition: no trigger carries Tf5DmgNoContract in the table")
		}
		tbl.Cards = cds
	})
	if n == 0 {
		t.Fatalf("no seed diverged with the contract gate dropped (spare %d)", tal.spare)
	}
}

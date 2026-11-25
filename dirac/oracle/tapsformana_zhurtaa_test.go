package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// Zhur-Taa Druid's "Whenever you tap CARDNAME for mana, it deals 1 damage to
// each opponent" is a self TapsForMana trigger (dirac/kgen
// lower_tapsformana.go). gorge's planner defers the Druid as a mana source
// (pay.PaymentPlanTapDamageTrigger accepts Mode$ Taps only), so the kernel
// must class the source Defer (dirac/mana_tapsdefer.go) and fire the damage
// line on the mana tap.
func zhurTaaDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Zhur-Taa Druid", Count: 4},
			{Name: "Grizzly Bears", Count: 4},
			{Name: "Giant Growth", Count: 4},
			{Name: "Forest", Count: 4},
			{Name: "Mountain", Count: 12},
			{Name: "Wastes", Count: 12},
		},
		{
			{Name: "Grizzly Bears", Count: 4},
			{Name: "Giant Growth", Count: 4},
			{Name: "Forest", Count: 16},
			{Name: "Mountain", Count: 16},
		},
	}
}

// zhurTaaRun plays the Druid's pool games and returns how many times the Druid
// was tapped for mana and how many seeds diverged. The game must cast it (its
// {R}{G} cost needs the Mountains) and then tap it, or the lockstep proves
// nothing.
func zhurTaaRun(t *testing.T, seeds uint64, mutate func(*kop.Table)) (taps, div int) {
	t.Helper()
	decks := zhurTaaDecks()
	reg, set, gd := lockstepPool(t, "tapsformana", decks)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"zhurtaa-a", "zhurtaa-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		finish := func(g *GorgeSide) {
			for _, e := range g.E.L.Events {
				if e.Kind != events.Tap {
					continue
				}
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Zhur-Taa Druid" {
					n++
				}
			}
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed),
			RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Finish: finish})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if res.Div != nil {
			div++
			if mutate == nil {
				t.Fatalf("seed %d diverged at decision %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
		}
		taps += n
	}
	return taps, div
}

// TestTapsForManaZhurTaaLockstep plays the Druid's pool games: no divergence,
// and the pool actually taps it for mana.
func TestTapsForManaZhurTaaLockstep(t *testing.T) {
	t.Parallel()
	taps, div := zhurTaaRun(t, 40, nil)
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if taps == 0 {
		t.Fatal("the pool never tapped Zhur-Taa Druid for mana: the lockstep proves nothing")
	}
}

// TestTapsForManaZhurTaaMutation drops the kernel's self TapsForMana line from
// every card: some seed diverges (gorge still deals the damage), proof the
// pool taps the Druid for mana and the lockstep exercises the line.
func TestTapsForManaZhurTaaMutation(t *testing.T) {
	t.Parallel()
	taps, div := zhurTaaRun(t, 40, zhurTaaDropSelfLine)
	if taps == 0 {
		t.Fatal("the pool never tapped Zhur-Taa Druid for mana: the mutation proves nothing")
	}
	if div == 0 {
		t.Fatal("no seed diverged with the kernel's self TapsForMana line dropped")
	}
}

// zhurTaaDropSelfLine removes every self Defer Taps line from the kernel's
// table (the mutation's "drop the line"), leaving the rest of each CardDef.
func zhurTaaDropSelfLine(tbl *kop.Table) {
	cs := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cs {
		abs := cs[i].Abilities
		kept := make([]kop.Ability, 0, len(abs))
		for _, a := range abs {
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps &&
				a.Trig.Flags&kop.TfSelf != 0 && a.Trig.TapPlan == kop.TapPlanDefer {
				continue
			}
			kept = append(kept, a)
		}
		cs[i].Abilities = kept
	}
	tbl.Cards = cs
}

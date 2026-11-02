package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: Mode$ DamageAll at a single player (Master of
// Barbs: ValidTarget$ Opponent, no trigger-context read), and
// DamageDoneOnce bodies that read the damaged referent (Cracked Skull:
// TriggeredTargetLKICopy of the enchanted damaged creature) or the
// attacking player (Contested Game Ball: TriggeredAttackingPlayer on
// combat damage to You). End the Festivities damages the opponent and each
// creature they control in one batch; Shock, Lightning Bolt, Burst
// Lightning and the creatures' combat damage supply the rest.
var kddoshapesDecks = [2][]cardset.Entry{
	{
		{Name: "Master of Barbs", Count: 4}, {Name: "Cracked Skull", Count: 3}, {Name: "Contested Game Ball", Count: 1},
		{Name: "End the Festivities", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Master of Barbs", Count: 3}, {Name: "Cracked Skull", Count: 3}, {Name: "Contested Game Ball", Count: 1},
		{Name: "Vampire Lacerator", Count: 3}, {Name: "Child of Night", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Burst Lightning", Count: 2},
		{Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 6},
	},
}

// kddoshapesTally counts the arms from gorge's event log, so the assertions
// below are not vacuous.
type kddoshapesTally struct {
	games   int
	mob     int // Master of Barbs DamageAll triggers
	sweep   int // the subset whose batch also damaged a creature
	skull   int // Cracked Skull DamageDoneOnce triggers
	control int // Contested Game Ball control changes
}

// addGame scans one finished game's event log. A damage batch is a window
// between the boundaries kddo's tally uses (priority, step, stack and turn):
// within one, Damage events accumulate for the referents the queue sees, and
// a Master of Barbs trigger is a sweep when a creature was also damaged.
func (p *kddoshapesTally) addGame(g *GorgeSide) {
	p.games++
	sawDamage, hadObj, lastHadObj := false, false, false
	flush := func() {
		if sawDamage {
			lastHadObj = hadObj
		}
		sawDamage, hadObj = false, false
	}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch e.Kind {
		case events.Damage:
			if e.Amount > 0 {
				sawDamage = true
				if e.Obj != 0 {
					hadObj = true
				}
			}
		case events.TriggerPush:
			name, tr := kddoMode(g.E.G, e.Obj, e.Amount)
			if tr == nil {
				continue
			}
			switch {
			case name == "Master of Barbs" && tr.Mode == "DamageAll":
				p.mob++
				// The queue runs at the priority after the batch, so the
				// trigger's batch is the last damage window, not the events
				// since the boundary.
				if lastHadObj {
					p.sweep++
				}
			case name == "Cracked Skull" && tr.Mode == "DamageDoneOnce":
				p.skull++
			}
		case events.ControlChange:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Contested Game Ball" {
				p.control++
			}
		case events.Priority, events.StepChange, events.TurnChange, events.PutOnStack,
			events.Resolve, events.DeclareAttackers, events.DeclareBlockers:
			flush()
		}
	}
	flush()
}

// kddoshapesRun plays the matchup over the pool. stopAtDiv returns at the
// first divergence without failing (a mutation test's proof); otherwise every
// divergence is reported and the run continues.
func kddoshapesRun(t *testing.T, seeds uint64, tally *kddoshapesTally, stopAtDiv bool) (games, div, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ddoshapes")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kddoshapesDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kddoshapesDecks[0], kddoshapesDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	clean := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kddoshapes-a", "kddoshapes-b"}, gd, 1)
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
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 60}
		if tally != nil {
			rc.Finish = tally.addGame
		}
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		games++
		if res.Stall != "" {
			stalls++
			if !stopAtDiv {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div++
			if stopAtDiv {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
		clean++
	}
	t.Logf("ddoshapes: %d seeds, %d clean, %d diverged, %d stalls", games, clean, div, stalls)
	return
}

// TestDdoshapesLockstep: 0 divergences and 0 stalls over 400 seeds, and the
// pool reaches all three lifted shapes.
func TestDdoshapesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var p kddoshapesTally
	_, div, stalls := kddoshapesRun(t, 400, &p, false)
	t.Logf("Master of Barbs triggers: %d (%d from a DamageAll sweep that also hit creatures); Cracked Skull destroys: %d; Contested Game Ball control changes: %d",
		p.mob, p.sweep, p.skull, p.control)
	if div != 0 {
		t.Errorf("%d divergences across 400 seeds", div)
	}
	if stalls != 0 {
		t.Errorf("%d stalls across 400 seeds", stalls)
	}
	for _, c := range []struct {
		name string
		n    int
		min  int
	}{
		{"Master of Barbs triggers", p.mob, 15},
		{"DamageAll sweeps that also hit creatures", p.sweep, 3},
		{"Cracked Skull destroys", p.skull, 5},
		{"Contested Game Ball control changes", p.control, 5},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.name, c.n, c.min)
		}
	}
}

// TestDdoshapesMutation: dropping the recipient filter
// (dirac.DebugDmgIgnoreFilter2) makes the lockstep diverge, so the counters
// above exercise both referent filters (Cracked Skull's damaged-object filter
// in particular).
func TestDdoshapesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDmgIgnoreFilter2 = true
	defer func() { dirac.DebugDmgIgnoreFilter2 = false }()
	_, div, _ := kddoshapesRun(t, 128, nil, true)
	t.Logf("ignore-filter2 mutation: %d diverged after at most 128 games", div)
	if div == 0 {
		t.Fatal("no seed diverged with the damage recipient filter dropped: the pool does not exercise it")
	}
}

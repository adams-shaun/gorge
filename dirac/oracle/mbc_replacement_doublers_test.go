package oracle

import (
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The MBC replacements-2 spelling family, one carrier per lowered spelling
// in the mbcrepl2 fixture pool (dirac/cardset/pools/fixtures/mbcrepl2):
//   - repl:DamageDone doubler ValidTarget "" (no ValidTarget$, any
//     recipient): Fire Servant doubles its controller's instant and sorcery
//     damage (Shock 4, Lightning Bolt 6, Lava Axe 10);
//   - repl:DamageDone doubler ValidSource "Card.Self" with IsCombat$ True
//     (kop.ReplDmgCombat): Charging Tuskodon's combat damage to a player is
//     doubled, its damage to a creature is not.
//
// Before the lowering both carriers were refused, so the matchup table did
// not build. The scan asserts each doubler did its thing in gorge's log.
// Laboratory Maniac stays refused (lower_drawrepl.go: gorge never replaces
// an empty-library draw).

var kRepl2Decks = [2][]cardset.Entry{
	{
		{Name: "Fire Servant", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Lava Axe", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 20},
	},
	{
		{Name: "Charging Tuskodon", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 8}, {Name: "Mountain", Count: 20},
	},
}

// repl2Tally counts, over gorge's DamageProvenance facts (the damage that
// landed, after replacements), what each doubler did.
type repl2Tally struct {
	SpellDoubled   int // a burn spell's damage above its printed amount (Fire Servant)
	SpellPlain     int // a burn spell's damage at its printed amount
	TuskPlayer2x   int // Charging Tuskodon's combat damage to a player above its power
	TuskCreatureHi int // Charging Tuskodon's damage to a creature above its power: must stay 0
}

// repl2Printed is each burn spell's printed damage.
var repl2Printed = map[string]int32{"Shock": 2, "Lightning Bolt": 3, "Lava Axe": 5}

func (c *repl2Tally) scan(g *GorgeSide) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.DamageProvenance || ev.Amount <= 0 || len(ev.IDs) == 0 {
			continue
		}
		src := g.E.G.Obj(ev.Obj)
		if src == nil || src.Face() == nil {
			continue
		}
		_, toPlayer := ev.IDs[0].PlayerRef()
		switch name := src.Face().Name; {
		case repl2Printed[name] != 0 && ev.Amount == 2*repl2Printed[name]:
			c.SpellDoubled++
		case repl2Printed[name] != 0 && ev.Amount == repl2Printed[name]:
			c.SpellPlain++
		case name == "Charging Tuskodon" && toPlayer && ev.Amount > 4:
			c.TuskPlayer2x++
		case name == "Charging Tuskodon" && !toPlayer && ev.Amount > 4:
			c.TuskCreatureHi++
		}
	}
}

func (c *repl2Tally) add(o repl2Tally) {
	c.SpellDoubled += o.SpellDoubled
	c.SpellPlain += o.SpellPlain
	c.TuskPlayer2x += o.TuskPlayer2x
	c.TuskCreatureHi += o.TuskCreatureHi
}

// repl2Probe is castFirstPolicy keeping the gorge side it was handed.
type repl2Probe struct {
	base Policy
	gs   *GorgeSide
}

func (p *repl2Probe) pick(c *PickCtx) Canon {
	if c.G != nil {
		p.gs = c.G
	}
	return p.base.Pick(c)
}

func TestMBCReplacements2Lowers(t *testing.T) {
	t.Parallel()
	seeds := uint64(160)
	if v := os.Getenv("KREPL2_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	reg, set, gd := lockstepPool(t, "mbcrepl2", kRepl2Decks)
	var tally repl2Tally
	clean := 0
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		tbl, _, _, err := set.Matchup(kRepl2Decks[0], kRepl2Decks[1])
		if err != nil {
			t.Fatal(err)
		}
		var local repl2Tally
		n := 0
		for seed := lo; seed <= hi; seed++ {
			m, err := NewMatchup(reg, [2]string{"mbcrepl2-a", "mbcrepl2-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			probe := &repl2Probe{base: castFirstPolicy(seed)}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
			}
			if d := res.Div; d != nil {
				from := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[from:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				continue
			}
			n++
			local.scan(probe.gs)
		}
		mu.Lock()
		defer mu.Unlock()
		clean += n
		tally.add(local)
	})
	if t.Failed() {
		return
	}
	t.Logf("%d seeds, %d clean; %+v", seeds, clean, tally)
	if tally.SpellPlain == 0 {
		t.Errorf("no burn spell dealt its printed damage: the doubled count proves nothing")
	}
	if tally.SpellDoubled == 0 {
		t.Errorf("Fire Servant never doubled a burn spell's damage")
	}
	if tally.TuskPlayer2x == 0 {
		t.Errorf("Charging Tuskodon never dealt doubled combat damage to a player")
	}
	if tally.TuskCreatureHi != 0 {
		t.Errorf("Charging Tuskodon dealt more than its power to a creature %d times", tally.TuskCreatureHi)
	}
}

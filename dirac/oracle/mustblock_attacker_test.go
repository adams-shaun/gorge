package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The api:MustBlock lockstep (dirac/kgen/lower_mustblock.go; kernel
// combat_mustblockatk.go): Crashing Boars' defending player picks one of
// their untapped creatures, Avalanche Tusker targets one, and that creature
// must block that attacker this turn (this combat) if able. gorge marks only
// the pairs naming the bound attacker Required (combat.MustBlockPairRequired)
// and accepts only a declaration that meets the most CR 509.1c requirements;
// the kernel must ask the same pick, offer the same pairs at every block pick
// and Done at the same point. Avalanche Tusker is in no named pool, so the
// table is built from the decks' own cards (printedDelveSet).
func kmustblockAtkDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Crashing Boars", Count: 4},
		{Name: "Avalanche Tusker", Count: 4},
		{Name: "Forest Bear", Count: 6},
		{Name: "Daggerback Basilisk", Count: 3},
		{Name: "Giant Spider", Count: 3},
		{Name: "Rampant Growth", Count: 3},
		{Name: "Forest", Count: 10},
		{Name: "Island", Count: 3},
		{Name: "Mountain", Count: 4},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

type kmustblockAtkStats struct {
	decls  int // block declarations offering a bound (BlockMust) pair
	multi  int // of them, with another attacker in combat the bound blocker must not be spent on
	tusker int // of them, bound to an attacker other than Crashing Boars (Avalanche Tusker's target)
}

// kmustblockAtkPolicy casts, attacks and blocks whenever it can (so the
// games reach the declarations), else random; it counts, from gorge's
// options, the declarations a bound duty decides.
func kmustblockAtkPolicy(st *kmustblockAtkStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DBlockers && !counted[c.Seq] {
				counted[c.Seq] = true
				kmustblockAtkCount(c, st)
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack, ActBlock:
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

func kmustblockAtkCount(c *PickCtx, st *kmustblockAtkStats) {
	bound, tusker := false, false
	attackers := map[state.ObjID]bool{}
	for _, x := range c.D.Options {
		if x.Kind() != ActBlock {
			continue
		}
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		attackers[o.Attacker] = true
		if o.BlockMust && !o.AttackMust {
			bound = true
			if ob := c.G.E.G.Obj(o.Attacker); ob != nil && ob.Face() != nil && ob.Face().Name != "Crashing Boars" {
				tusker = true
			}
		}
	}
	if bound {
		st.decls++
		if len(attackers) > 1 {
			st.multi++
		}
		if tusker {
			st.tusker++
		}
	}
}

func TestMustBlockLockstep(t *testing.T) {
	t.Parallel()
	decks := kmustblockAtkDecks()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	var st kmustblockAtkStats
	clean, _, div := lockstepSeeds(t, reg, set, "mustblock", decks, gd, 1, 120, true, nil, nil, kmustblockAtkPolicy(&st))
	t.Logf("%d seeds clean, %d diverged; %d declarations with a bound blocker, %d of them with another attacker in combat, %d bound by Avalanche Tusker", clean, div, st.decls, st.multi, st.tusker)
	if st.decls < 5 || st.multi < 3 || st.tusker < 3 {
		t.Fatalf("only %d declarations with a bound blocker (%d with another attacker, %d by Avalanche Tusker); the lockstep proves nothing", st.decls, st.multi, st.tusker)
	}
}

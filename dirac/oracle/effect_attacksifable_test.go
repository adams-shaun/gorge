package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The effect-attacks-if-able lockstep: Heckling Fiends' "{2}{R}: Target
// creature attacks this turn if able." and Incite's "Target creature
// becomes red until end of turn and attacks this turn if able." (the
// Effect reads its parent's target) (CR 508.1d; gorge Effect Mode$
// MustAttack over the remembered target). gorge marks the creature's attack
// options Required; the kernel (ExtAttacksIfAble, combat_attacksifable.go)
// must declare it (auto-declared before the decision in the default
// configuration) and free it the next turn. No other card here makes a
// creature attack, so every Required attack option is a mark.
var keffmaDecks = [2][]cardset.Entry{
	{
		{Name: "Heckling Fiends", Count: 8}, {Name: "Incite", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 3}, {Name: "Mountain", Count: 17},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 6},
		{Name: "Heckling Fiends", Count: 4}, {Name: "Mountain", Count: 16},
	},
}

type keffmaStats struct {
	marked int // attackers decisions with a Required (marked) creature offered
	freed  int // a creature marked on an earlier turn offered as an attacker with no requirement
}

// keffmaPolicy casts, activates and plays lands whenever it can and attacks
// on every other attack pick only (so unmarked creatures stay home and a
// marked creature's declaration is the requirement's), else random; it
// counts, from gorge's options, the declarations the mark decides.
func keffmaPolicy(st *keffmaStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		var buf []Canon
		marked := map[state.ObjID]int{} // attacker -> turn it last had a mark
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DAttackers {
				keffmaCount(c, st, marked)
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand:
					buf = append(buf, a)
				case ActAttack:
					if c.Seq%2 == 0 {
						buf = append(buf, a)
					}
				}
			}
			if len(buf) == 0 {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

func keffmaCount(c *PickCtx, st *keffmaStats, marked map[state.ObjID]int) {
	turn := c.G.Turn()
	has := false
	plain := map[state.ObjID]bool{}
	for _, x := range c.D.Options {
		if x.Kind() != ActAttack {
			continue
		}
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		if o.Required {
			has = true
			marked[o.Obj] = turn
		} else {
			plain[o.Obj] = true
		}
	}
	for a := range plain {
		if t, ok := marked[a]; ok && t < turn {
			st.freed++
			delete(marked, a)
		}
	}
	if has {
		st.marked++
	}
}

func TestEffectAttacksIfAbleLockstep(t *testing.T) {
	t.Parallel()
	var st keffmaStats
	lockstepRunPolicy(t, "attacksifable", keffmaDecks, 400, true, nil, nil, keffmaPolicy(&st))
	t.Logf("%d attackers decisions with a marked creature, %d marks seen expired", st.marked, st.freed)
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.marked, "attackers decisions with a marked creature"},
		{st.freed, "creatures freed of their mark the next turn"},
	} {
		if c.n < 5 {
			t.Errorf("only %d %s, want >= 5", c.n, c.what)
		}
	}
}

// TestEffectAttacksIfAbleLockstepMutation: with the attack mark dropped,
// some seed diverges.
func TestEffectAttacksIfAbleLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "attacksifable", keffmaDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpExt && p[x].Ext() == kop.ExtAttacksIfAble {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the attack mark dropped")
	}
}

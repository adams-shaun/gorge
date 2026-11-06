package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The effect-pair lockstep: Hustle // Bustle's Hustle half is "Target
// creature attacks or blocks this turn if able" (the split card is named by
// its front face, Hustle): one Effect over one
// remembered target with two SVar statics, Mode$ MustAttack and Mode$
// MustBlock (CR 508.1d, 509.1c). gorge registers both duties on the target;
// the kernel must mark the same creature with ExtAttacksIfAble and
// ExtBlocksIfAble (combat_attacksifable.go, combat_blocksifable.go), offer
// the same Required attack and BlockMust pairs, and free both the next turn.
// Either duty alone is a divergence.
var keffpairDecks = [2][]cardset.Entry{
	{
		{Name: "Hustle", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Raging Goblin", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Island", Count: 12},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 6},
		{Name: "Hustle", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 10},
	},
}

type keffpairStats struct {
	attack      int // attackers decisions with a Required (marked) creature offered
	attackFreed int // a creature marked for attack on an earlier turn offered with no requirement
	block       int // blocker decisions with a BlockMust (duty) pair offered
	blockFreed  int // a creature with a block duty on an earlier turn offered a block with no duty
}

// keffpairPolicy casts, activates and plays lands whenever it can, attacks on
// every other attack pick (so pair-marked creatures are sometimes home to
// block), else random; it counts, from gorge's options, both declarations the
// pair decides.
func keffpairPolicy(st *keffpairStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		var buf []Canon
		attackDuty := map[state.ObjID]int{} // attacker -> turn it last had an attack mark
		blockDuty := map[state.ObjID]int{}  // blocker -> turn it last had a block duty
		attackPicks := 0                    // attackers decisions so far
		return PolicyFunc(func(c *PickCtx) Canon {
			attack := false
			if c.G != nil {
				switch c.D.Kind {
				case DAttackers:
					attack = attackPicks%2 == 0
					attackPicks++
					keffpairAttackCount(c, st, attackDuty)
				case DBlockers:
					keffpairBlockCount(c, st, blockDuty)
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand:
					buf = append(buf, a)
				case ActAttack:
					// every other attack pick only, so creatures stay back
					// untapped to block on the turns after their pair
					if attack {
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

func keffpairAttackCount(c *PickCtx, st *keffpairStats, marked map[state.ObjID]int) {
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
			st.attackFreed++
			delete(marked, a)
		}
	}
	if has {
		st.attack++
	}
}

func keffpairBlockCount(c *PickCtx, st *keffpairStats, duty map[state.ObjID]int) {
	turn := c.G.Turn()
	has := false
	plain := map[state.ObjID]bool{}
	for _, x := range c.D.Options {
		if x.Kind() != ActBlock {
			continue
		}
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		if o.BlockMust {
			has = true
			duty[o.Obj] = turn
		} else {
			plain[o.Obj] = true
		}
	}
	for b := range plain {
		if t, ok := duty[b]; ok && t < turn {
			st.blockFreed++
			delete(duty, b)
		}
	}
	if has {
		st.block++
	}
}

func TestEffectPairHustleLockstep(t *testing.T) {
	t.Parallel()
	var st keffpairStats
	lockstepRunPolicy(t, "effectpairhustle", keffpairDecks, 400, true, nil, nil, keffpairPolicy(&st))
	t.Logf("%d attackers decisions with a marked creature, %d attack marks expired, %d blocker decisions with a duty, %d block duties expired",
		st.attack, st.attackFreed, st.block, st.blockFreed)
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.attack, "attackers decisions with a marked creature"},
		{st.attackFreed, "creatures freed of their attack mark the next turn"},
		{st.block, "blocker decisions with a duty pair"},
		{st.blockFreed, "creatures freed of their block duty the next turn"},
	} {
		if c.n < 5 {
			t.Errorf("only %d %s, want >= 5", c.n, c.what)
		}
	}
}

// TestEffectPairHustleLockstepMutation: with both pair marks dropped, some
// seed diverges.
func TestEffectPairHustleLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "effectpairhustle", keffpairDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpExt && (p[x].Ext() == kop.ExtAttacksIfAble || p[x].Ext() == kop.ExtBlocksIfAble) {
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
		t.Fatal("no seed diverged with both pair marks dropped")
	}
}

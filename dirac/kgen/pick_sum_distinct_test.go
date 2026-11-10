package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// builtInstrs visits every instruction of every built card in kp.
func builtInstrs(kp kfhPool, fn func(in kop.Instr)) {
	for _, b := range kp.built {
		for i := range b.def.Abilities {
			for _, in := range b.def.Abilities[i].Prog {
				fn(in)
			}
		}
	}
}

// TestPickTotalCMC: Lively Dirge's hidden graveyard pick (ChangeNum 2) and
// Michelangelo's Technique's Dig carry WithTotalCMC$ on the pick filter's
// head, the table field the kernel's pickFits reads (kop.Filter.PickSumMax).
// A budget the kernel cannot carry as a positive constant, or one beside the
// distinct-name walk, refuses by name.
func TestPickTotalCMC(t *testing.T) {
	const dirge, michel = "Lively Dirge", "Michelangelo's Technique"
	kp := kfhBuild(t, []string{dirge, michel}, nil)
	for _, n := range []string{dirge, michel} {
		kp.accepted(t, n)
	}
	var digBudget, pickBudget int16
	builtInstrs(kp, func(in kop.Instr) {
		f := kp.pt.filters[in.F]
		switch in.Op {
		case kop.OpDigEx:
			digBudget = f.PickSumMax.N
			if f.PickSumMax.K != kop.VConst {
				t.Errorf("Dig budget kind %v, want a constant", f.PickSumMax.K)
			}
		case kop.OpChooseMove:
			pickBudget = f.PickSumMax.N
		}
	})
	if digBudget != 6 || pickBudget != 4 {
		t.Fatalf("Dig budget %d, hidden pick budget %d; want 6 and 4", digBudget, pickBudget)
	}

	// the budget must be a constant: an SVar or X cannot ride table data
	for _, v := range []string{"X", "Count$Valid Creature.YouCtrl"} {
		kp := kfhBuild(t, []string{michel}, func(p *pool) {
			editDigParam(p, michel, "WithTotalCMC", v)
		})
		kp.refusedWith(t, michel, "WithTotalCMC$")
	}
	// the distinct-name walk owns its pick-set state, so the two cannot share
	// a filter head
	kp = kfhBuild(t, []string{michel}, func(p *pool) {
		editDigParam(p, michel, "DifferentNames", "True")
	})
	kp.refusedWith(t, michel, "WithTotalCMC$ beside DifferentNames$")
}

// editDigParam rewrites one parameter of the card's Dig ability (a copy: the
// corpus card is shared). A spell's Dig is a printed ability; a permanent's
// is a trigger body, whose Effect pointer the shallow face copy shares, so
// that one is copied out.
func editDigParam(p *pool, name, key, val string) {
	c := kfhClone(p.byName[name])
	f := c.Faces[0]
	for _, a := range f.Abilities {
		if a.API == "Dig" {
			a.Params[key] = val
		}
	}
	f.Triggers = append([]cards.Trigger(nil), f.Triggers...)
	for i := range f.Triggers {
		old := f.Triggers[i].Effect
		if old == nil || old.API != "Dig" {
			continue
		}
		e := &cards.SA{Kind: old.Kind, API: old.API, Params: map[string]string{}}
		for k, v := range old.Params {
			e.Params[k] = v
		}
		e.Params[key] = val
		f.Triggers[i].Effect = e
	}
	p.byName[name] = c
}

// TestPickDifferentPowers: Rip, Spawn Hunter's Dig takes creature and
// Vehicle cards with pairwise different powers; the flag rides the filter
// head (kop.Filter.PickDistinctPower). Any other spelling refuses.
func TestPickDifferentPowers(t *testing.T) {
	const rip = "Rip, Spawn Hunter"
	kp := kfhBuild(t, []string{rip}, nil)
	kp.accepted(t, rip)
	var distinct, digs int
	builtInstrs(kp, func(in kop.Instr) {
		if in.Op != kop.OpDigEx {
			return
		}
		digs++
		if kp.pt.filters[in.F].PickDistinctPower {
			distinct++
		}
	})
	if digs != 1 || distinct != 1 {
		t.Fatalf("%d Dig instructions, %d with distinct powers; want 1 and 1", digs, distinct)
	}

	kp = kfhBuild(t, []string{rip}, func(p *pool) {
		editDigParam(p, rip, "WithDifferentPowers", "False")
	})
	kp.refusedWith(t, rip, "Dig WithDifferentPowers$")
}

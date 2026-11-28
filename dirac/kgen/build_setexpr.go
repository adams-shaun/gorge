package kgen

import (
	"fmt"
	"io"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The builder half of the Defined$ set expressions (lower_definedset.go): an
// instruction whose subject is rSet gets Obj kop.RefSet and a kop.SetRef
// entry for its PC, naming the card's set interned into Table.Sets. Every
// shape the kernel's execSet would panic on refuses here instead.

// kopTakesSet reports whether op is one dirac's execSet serves (dirac
// value_setexpr.go setOp; keep the two lists equal).
func kopTakesSet(op kop.Op) bool {
	switch op {
	case kop.OpExile, kop.OpBounce, kop.OpReturn, kop.OpToLibrary,
		kop.OpPumpUntilEOT, kop.OpPutCounter, kop.OpDealDamage, kop.OpDestroy, kop.OpSacrifice,
		kop.OpTap, kop.OpUntap, kop.OpGainLife, kop.OpLoseLife, kop.OpDraw, kop.OpDiscard,
		kop.OpGainControl, kop.OpGainControlEOT, kop.OpAnimate:
		return true
	}
	return false
}

// setSubjRef is an rSet's kop ref: RefSet as an instruction's subject, a
// refusal anywhere else (a value, a condition, a side input).
func (cb *cardBuilder) setSubjRef() (kop.Ref, error) {
	if !cb.setSubj {
		return 0, fmt.Errorf("a Defined$ set outside an instruction's subject")
	}
	return kop.RefSet, nil
}

// abilityProg builds an ability's own program, the one program whose
// instructions may read a set: their SetRefs ride ka.
func (cb *cardBuilder) abilityProg(p []lInstr, ka *kop.Ability) error {
	prev := cb.setRefs
	cb.setRefs = &ka.SetRefs
	defer func() { cb.setRefs = prev }()
	var err error
	if ka.Prog, err = cb.prog(p); err != nil {
		return err
	}
	if len(ka.SetRefs) > 0 && int(ka.NTargets) > kop.MaxTargets-1 {
		return fmt.Errorf("Defined$ set in an ability that uses the kernel's last target slot %d", kop.MaxTargets-1)
	}
	return nil
}

// setRef binds the set instruction pc of the program being built reads.
func (cb *cardBuilder) setRef(pc int, in lInstr, ki *kop.Instr) error {
	switch {
	case cb.setRefs == nil:
		return fmt.Errorf("Defined$ set outside an ability's own program")
	case !kopTakesSet(ki.Op):
		return fmt.Errorf("Defined$ set on kop %s", ki.Op)
	case ki.Op == kop.OpReturn:
		// a returning Aura's enchant ask re-reads the instruction's own
		// subject, not the member execSet bound (dirac returnrem.go
		// returningObj): the kernel would index RefSet as an object
		return fmt.Errorf("Defined$ set on kop %s: the enchant ask reads the instruction's subject", ki.Op)
	case ki.Obj != kop.RefSet:
		return fmt.Errorf("Defined$ set the %s form does not read as its subject", in.Op)
	case pc > 0xff:
		return fmt.Errorf("Defined$ set at pc %d", pc)
	}
	id, err := cb.internSet(in.Obj.N)
	if err != nil {
		return err
	}
	*cb.setRefs = append(*cb.setRefs, kop.SetRef{PC: uint8(pc), Set: id})
	return nil
}

// internSet builds the card's set n and interns it into the pool's Sets by
// structural equality, in first-use order.
func (cb *cardBuilder) internSet(n int) (uint16, error) {
	if n < 0 || n >= len(cb.c.Sets) {
		return 0, fmt.Errorf("Defined$ set %d of a card with %d", n, len(cb.c.Sets))
	}
	e := cb.c.Sets[n].kopSetExpr()
	for i, op := range cb.c.Sets[n].Ops {
		if op.F == nil {
			continue
		}
		id, err := cb.filter(op.F, kop.ZoneBattlefield) // no zone gate: the set's objects are anywhere
		if err != nil {
			return 0, err
		}
		e[i].Arg = uint16(id)
	}
	if d, ok := e.SetDepth(); !ok || d > kop.MaxSetDepth {
		return 0, fmt.Errorf("Defined$ set program %v (depth %d)", e, d)
	}
	key := fmt.Sprint(e)
	pt := cb.pt
	if id, ok := pt.setKey[key]; ok {
		return id, nil
	}
	if len(pt.sets) >= 0xffff {
		return 0, fmt.Errorf("%s more than %d set expressions", tableLimitMark, 0xffff)
	}
	if pt.setKey == nil {
		pt.setKey = map[string]uint16{}
	}
	id := uint16(len(pt.sets))
	pt.sets = append(pt.sets, e)
	pt.setKey[key] = id
	return id, nil
}

// setFilters calls filt on every filter the sets ab reads name.
func setFilters(pt *poolTables, ab *kop.Ability, filt func(kop.FilterID)) {
	for _, r := range ab.SetRefs {
		for _, op := range pt.sets[r.Set] {
			if op.Kind == kop.SetAndFilter {
				filt(kop.FilterID(op.Arg))
			}
		}
	}
}

// dumpSets is the card's SET lines, so its LIR key covers its programs.
func dumpSets(w io.Writer, c *lCard) {
	for i, s := range c.Sets {
		fmt.Fprintf(w, "  SET %d %s\n", i, s)
	}
}

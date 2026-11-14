package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// instrCtx is one LIR instruction on its way to its kop form: the
// instruction, the kop.Instr being built, and the helpers every op handler
// shares. (*cardBuilder).instr (build_ab.go) fills the shared operands and
// runs the op's handler.
type instrCtx struct {
	cb *cardBuilder
	in lInstr
	ki kop.Instr
	fl map[string]string // the instruction's CS flags (flagsOf)
	// skipFilterCheck: the handler emitted a form that drops the LIR filter
	// on purpose (DamageAll's each-player and each-opponent forms), so instr
	// does not refuse the instruction for a filter it did not emit.
	skipFilterCheck bool
}

// instrOps maps an LIR op name to its handler. Each instr_<family>.go
// registers its ops in init(); a new op is a new handler and one instrOp line
// in its family's file (or a new file), never an edit to a shared switch.
var instrOps = map[string]func(*instrCtx) error{}

// instrOp registers the handler for the LIR op name; a second registration of
// one name panics at init.
func instrOp(name string, h func(*instrCtx) error) {
	if _, dup := instrOps[name]; dup {
		panic("kgen: LIR op " + name + " registered twice")
	}
	instrOps[name] = h
}

// skip is the instruction's skip count as a constant operand.
func (ic *instrCtx) skip() kop.ValueRef { return kop.Const(ic.in.Skip) }

// objFilter emits the instruction's object filter, read in zone.
func (ic *instrCtx) objFilter(zone kop.Zone) error {
	var err error
	ic.ki.F, err = ic.cb.filter(ic.in.F, zone)
	return err
}

// moveFlags packs the zone-move flags the CS flags name.
func (ic *instrCtx) moveFlags() int {
	fl := ic.fl
	m := 0
	if _, ok := fl["Remember"]; ok {
		m |= kop.MvRemember
	}
	if _, ok := fl["ImprintMove"]; ok {
		m |= kop.MvImprint // lower_imprintzone.go
	}
	if _, ok := fl["Tapped"]; ok {
		m |= kop.MvTapped
	}
	if _, ok := fl["UnderYourControl"]; ok {
		m |= kop.MvUnderYourControl
	}
	if _, ok := fl["Transformed"]; ok {
		m |= kop.MvTransformed
	}
	if _, ok := fl["ForgetOther"]; ok {
		m |= kop.MvForgetOther
	}
	if _, ok := fl["LKI"]; ok {
		m |= kop.MvLKI
	}
	if _, ok := fl["Attacking"]; ok {
		m |= kop.MvAttacking
	}
	if _, ok := fl["AlsoSelf"]; ok {
		m |= kop.MvAlsoSelf
	}
	if _, ok := fl["ForgetChanged"]; ok {
		m |= kop.MvForgetChanged
	}
	return m
}

// counters reads the counters=<kind>[,<kind>] flag into A (the count, 1 when
// the LIR gives none) and B (the counter kind).
func (ic *instrCtx) counters() error {
	if k, ok := ic.fl["counters"]; ok {
		if strings.Contains(k, ",") {
			return ic.cb.counterPair(&ic.ki, ic.in, k) // lower_counterlist.go
		}
		ck, err := counterKind(k)
		if err != nil {
			return err
		}
		if ic.in.A.Kind == vNone {
			ic.ki.A = kop.Const(1)
		}
		ic.ki.B = kop.Const(int(ck))
	}
	return nil
}

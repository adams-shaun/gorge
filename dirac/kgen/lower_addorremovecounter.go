package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// apiAddOrRemoveCounter lowers Forge's AddOrRemoveCounter ("choose a counter
// on target permanent: remove it or put another on it"). The shapes:
//
//   - CounterType$ names one kind (Plague Boiler, Sigurd): the election is
//     one add/remove ask for that kind.
//   - EachExistingCounter$ True (Quarry Hauler, Dramatist's Puppet): the
//     target's counter kinds are walked in turn, one add/remove ask each.
//
// Gorge's other shapes are refused here by name: RemoveConditionSVar$ picks
// the arm from a condition (Etched Host Doombringer, Portent Tracker, Shape of
// the Wiitigo), DefinedPlayer$ asks a different player (Lavabrink
// Floodgates), Optional$ adds a skip option (Lavabrink Floodgates), a bare
// CounterType$ with each-existing also set is an unimplemented gorge shape,
// and no CounterType$ at all is Clockspinning's combined pick over every
// counter kind. RememberRemovedCards$ (Guile) is a remembered-card write the
// kernel op does not carry.
func (lc *lowerCtx) apiAddOrRemoveCounter(sa *cards.SA, ps *params) error {
	if cond := ps.str("RemoveConditionSVar"); cond != "" {
		return refuse("AddOrRemoveCounter RemoveConditionSVar$ %q (the condition picks the arm)", cond)
	}
	if spec := ps.str("DefinedPlayer"); spec != "" {
		return refuse("AddOrRemoveCounter DefinedPlayer$ %q (another player's ask)", spec)
	}
	if ps.is("Optional", "True") {
		return refuse("AddOrRemoveCounter Optional$ (a skip option the kernel op does not pose)")
	}
	if ps.is("RememberRemovedCards", "True") {
		return refuse("AddOrRemoveCounter RememberRemovedCards$ (no remembered-card write)")
	}
	if d := ps.str("Defined"); strings.HasPrefix(d, "Valid") {
		return refuse("AddOrRemoveCounter Defined$ %q (a sweep; gorge acts on the first object)", d)
	}
	if z := ps.str("TgtZone"); z != "" && z != "Battlefield" {
		return refuse("AddOrRemoveCounter TgtZone$ %q", z)
	}
	named := ps.str("CounterType")
	eachExisting := ps.is("EachExistingCounter", "True")
	if eachExisting && named != "" {
		return refuse("AddOrRemoveCounter EachExistingCounter$ with CounterType$ %q", named)
	}
	if !eachExisting && named == "" {
		return refuse("AddOrRemoveCounter without CounterType$ (the combined kind pick)")
	}
	if strings.Contains(named, ",") {
		return refuse("AddOrRemoveCounter CounterType$ %q (a kind choice)", named)
	}
	if !eachExisting {
		if _, err := counterKind(named); err != nil {
			return refuse("AddOrRemoveCounter CounterType$ %q: %v", named, err)
		}
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "CounterNum", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "AddOrRemoveCounter", Obj: obj, A: n, CS: named}
	if eachExisting {
		in.C = -1
		in.Note = "for each kind of counter on it"
	}
	lc.emit(in)
	return nil
}

// opAddOrRemoveCounter builds OpExt ExtAddOrRemoveCounter: Obj the target or
// Defined$ object, B the amount, C the named counter kind or -1 for
// EachExistingCounter$.
func (ic *instrCtx) opAddOrRemoveCounter() error {
	ki := &ic.ki
	in := &ic.in
	v, err := ic.cb.value(in.A)
	if err != nil {
		return err
	}
	ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtAddOrRemoveCounter)), v
	ki.C = int16(in.C)
	if in.C >= 0 {
		k, err := counterKind(in.CS)
		if err != nil {
			return err
		}
		ki.C = int16(k)
	}
	return nil
}

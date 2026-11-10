package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Count$CountersAddedThisTurn <Kind> You <Spec> as ExCtrAdded.
//
// gorge (engine/rules/emit.go) appends {adder, kind, amount, pre-Apply
// object snapshot} to a per-turn ledger for every positive CounterChange
// that is not an internal marker and whose adder is known
// (inFlightCounterAdder: the in-flight publication, else the resolving
// stack object's controller, else a replacement body's source controller;
// the kernel's counterAdder mirrors it, 255 for none). The ledger resets at
// TurnChange. Every carrier reads the head under CheckSVar$ X (>= 1), so the
// kernel keeps presence, not amounts:
//
//   - genState.ctrAdd.pl bit p: player p put a counter on a creature (the
//     Creature form; the type is read at placement, as the snapshot is);
//   - genState.ctrAdd.e: one record per OBJECT, only for a card that reads a
//     Self form (gfCtrAdd), keyed by Obj: gorge matches
//     Card.Self against the snapshot by object id, and an object keeps its
//     id through a zone change, so a flickered source still finds its old
//     record (the kernel's Obj persists too). fl is the kind-by-player mask.
//
// Every record is behind prep.anyCtrAdded, a table property.

// StallCtrAdded: a fifth object with a Self reader got a counter in one turn.
// A read the walk did not provide for is StallLowering.
const StallCtrAdded = 91

func init() { stallNames[StallCtrAdded] = "ctradded" }

// DebugCtrAddedNoTurnReset is the mutation switch of TestCtraddedMutation:
// the record survives the turn boundary.
var DebugCtrAddedNoTurnReset bool

// prepCtrAdded sets prep.anyCtrAdded and flags every card that reads an
// ExCtrAdded Self form through a trigger, static or
// replacement condition.
func prepCtrAdded(t *kop.Table, p *prep, g *genPrep) {
	any := false
	for i := range t.Exprs {
		any = any || t.Exprs[i].Kind == kop.ExCtrAdded
	}
	if !any {
		return
	}
	p.anyCtrAdded = true
	for c := range t.Cards {
		ab := t.Cards[c].Abilities
		for i := range ab {
			if condReadsCtrSelf(t, ab[i].Trig.Cond) || condReadsCtrSelf(t, ab[i].Static.Cond2) || condReadsCtrSelf(t, ab[i].Repl.Cond) {
				g.setFlag(CardID(c), gfCtrAdd)
				if MaxSeats > 2 {
					g.twoSeatOnly = true // ctrAdd.e[].fl packs two seats per kind
				}
			}
		}
	}
}

func condReadsCtrSelf(t *kop.Table, id kop.CondID) bool {
	if id == 0 || int(id) >= len(t.Conds) {
		return false
	}
	c := &t.Conds[id]
	return valReadsCtrSelf(t, c.A) || valReadsCtrSelf(t, c.B) || condReadsCtrSelf(t, c.Sub[0]) || condReadsCtrSelf(t, c.Sub[1])
}

func valReadsCtrSelf(t *kop.Table, v kop.ValueRef) bool {
	if v.K != kop.VExpr || v.N <= 0 || int(v.N) >= len(t.Exprs) {
		return false
	}
	x := &t.Exprs[v.N]
	if x.Kind == kop.ExCtrAdded {
		return x.N>>kop.CtrAddedFormShift != kop.CtrAddedFormCreature
	}
	return valReadsCtrSelf(t, x.Sub[0]) || valReadsCtrSelf(t, x.Sub[1])
}

// ctrNote records a counter of kind put on the permanent at sl by the
// player counterAdder names. Callers guard with k.anyCtrAdded.
//
//go:noinline
func (s *State) ctrNote(k *prep, sl Slot, kind int) {
	s.ctrNoteBy(k, sl, kind, s.counterAdder())
}

// ctrNoteBy is ctrNote with the adder given (a cost, an infect source).
//
//go:noinline
func (s *State) ctrNoteBy(k *prep, sl Slot, kind int, adder uint8) {
	if adder > 1 || !kop.IsCounterKind(kind) {
		return
	}
	g := &s.g.ctrAdd
	// Creature is read from the permanent's type at placement; gorge matches
	// it against the snapshot with layered types, so an animated land or a
	// crewed Vehicle could differ (no pool card exercises it)
	if s.isCreature(k, sl) {
		g.pl |= 1 << adder
	}
	if !k.gen.has(s.cardAt(sl), gfCtrAdd) {
		return
	}
	fl := uint8(1) << adder
	switch kop.CounterKind(kind) {
	case kop.CtrP1P1:
		fl |= 4 << adder
	case kop.CtrLore:
		fl |= 16 << adder
	}
	o := s.perms[sl].obj
	for i := uint8(0); i < g.n; i++ {
		if g.e[i].o == o {
			g.e[i].fl |= fl
			return
		}
	}
	if int(g.n) == len(g.e) {
		s.capFull(StallCtrAdded)
		return
	}
	g.e[g.n].o, g.e[g.n].fl = o, fl
	g.n++
}

// ctrNoteEnter records the counters a permanent entered with (gorge's
// EntryCounterNotice: the adder is the entrant's controller).
//
//go:noinline
func (s *State) ctrNoteEnter(k *prep, sl Slot) {
	p := &s.perms[sl]
	for c := 0; c < int(kop.NumCounterKinds3); c++ {
		if kop.IsCounterKind(c) && p.ctrN(s, c) > 0 {
			s.ctrNoteBy(k, sl, c, p.ctrl)
		}
	}
}

// ctrAddedReset ends the turn's record (newTurnGen).
//
//go:noinline
func (s *State) ctrAddedReset() {
	if !DebugCtrAddedNoTurnReset {
		s.g.ctrAdd.pl, s.g.ctrAdd.n = 0, 0
		s.g.ctrAdd.e = [4]struct {
			o  Obj
			fl uint8
		}{}
	}
}

// ctrAdded evaluates ExCtrAdded: 1 when the reading controller put a
// matching counter this turn.
//
//go:noinline
func (s *State) ctrAdded(k *prep, x *kop.Expr, ctx evalCtx) int {
	p := SeatIx(ctx.ctrl)
	g := &s.g.ctrAdd
	form := int(x.N) >> kop.CtrAddedFormShift
	if form == kop.CtrAddedFormCreature {
		if g.pl&(1<<p) != 0 {
			return 1
		}
		return 0
	}
	if ctx.src == NoObj {
		s.capFull(StallLowering) // an object form read with no source object (an emblem static) would read a record never keyed
		return 0
	}
	if k.gen == nil || !k.gen.has(s.objs[ctx.src].card, gfCtrAdd) {
		s.capFull(StallLowering) // a reader the prep walk did not flag would read a record never kept
		return 0
	}
	var bit uint8
	switch int(x.N) & kop.CtrAddedKindMask {
	case kop.CtrAddedKindP1P1:
		bit = 4 << p
	case kop.CtrAddedKindLore:
		bit = 16 << p
	default:
		bit = 1 << p
	}
	for i := uint8(0); i < g.n; i++ {
		if g.e[i].o == ctx.src {
			if g.e[i].fl&bit != 0 {
				return 1
			}
			return 0
		}
	}
	return 0
}

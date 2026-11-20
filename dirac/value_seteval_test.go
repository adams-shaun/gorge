package dirac

import (
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The SetExpr tests build their tables by hand: no card of any pool lowers to
// RefSet yet (T07), so every table here is vanilla's plus a few test cards.

// setTbl is vanilla's table with the filters and sets appended (a copy: v.Table
// is shared), and the ids of the appended cards.
type setTbl struct {
	t     *kop.Table
	fCre  kop.FilterID // creatures you control
	fAnyC kop.FilterID // any creature
	cards []CardID
}

func newSetTbl(t *testing.T, sets []kop.SetExpr, cards ...kop.CardDef) *setTbl {
	t.Helper()
	tbl, ids := kcapsTable(t, cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	st := &setTbl{t: tbl, cards: ids}
	st.fCre = kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	st.fAnyC = kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	tbl.Sets = append([]kop.SetExpr{{}}, sets...)
	return st
}

// setPos is a hand-built position: seat 0 has creatures A, B and land L0, seat 1
// creature C and land L1, created in that order.
type setPos struct {
	sc              *scen
	a, b, l0, c, l1 Obj
}

func newSetPos(t *testing.T, st *setTbl) *setPos {
	t.Helper()
	sc := newScenTable(t, st.t, Config{NoElide: true})
	p := &setPos{sc: sc}
	p.a = sc.perm(v.GrizzlyBears, 0)
	p.l0 = sc.perm(v.Plains, 0)
	p.b = sc.perm(v.GrizzlyBears, 0)
	p.c = sc.perm(v.GrizzlyBears, 1)
	p.l1 = sc.perm(v.Plains, 1)
	return p
}

// ctxFor is an evaluation context for controller ctrl whose entry targets t0.
func (p *setPos) ctxFor(ctrl uint8, t0 Obj) evalCtx {
	e := &stackEntry{ctrl: ctrl, obj: NoObj}
	for i := range e.tgt {
		e.tgt[i] = TCode(NoTarget)
	}
	e.tgt[0] = TCode(t0)
	return evalCtx{src: NoObj, srcSl: NoSlot, ctrl: ctrl, e: e}
}

func members(v *setVal) []Obj {
	out := make([]Obj, v.n)
	for i := uint16(0); i < v.n; i++ {
		out[i] = Obj(v.objs[i])
	}
	return out
}

// TestEvalSetBases: each base and combinator, on a hand-built position, gives
// the expected members in the set's order.
func TestEvalSetBases(t *testing.T) {
	bf := int16(kop.ZoneBattlefield)
	var st *setTbl
	mk := func(order kop.SetOrder, players bool, ops func(*setTbl) []kop.SetOp) kop.SetExpr {
		return kop.SetExpr{Ops: ops(st), Players: players, Order: order}
	}
	// the filters exist only once the table does: the ops are built from st
	st = newSetTbl(t, nil)
	sets := []kop.SetExpr{
		0: {},
		// 1: target 0
		1: mk(kop.OrderList, false, func(*setTbl) []kop.SetOp { return []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefTarget0}} }),
		// 2: creatures you control
		2: mk(kop.OrderObj, false, func(s *setTbl) []kop.SetOp { return []kop.SetOp{{Kind: kop.SoBaseValid, F: s.fCre, Arg: bf}} }),
		// 3: every permanent, then the creatures
		3: mk(kop.OrderObj, false, func(s *setTbl) []kop.SetOp {
			return []kop.SetOp{{Kind: kop.SoBaseValid, Arg: bf}, {Kind: kop.SoInter, F: s.fAnyC}}
		}),
		// 4: target 0 then your creatures, list order
		4: mk(kop.OrderList, false, func(s *setTbl) []kop.SetOp {
			return []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefTarget0}, {Kind: kop.SoBaseValid, F: s.fCre, Arg: bf}, {Kind: kop.SoUnion}}
		}),
		// 5: the same union, object order
		5: mk(kop.OrderObj, false, func(s *setTbl) []kop.SetOp {
			return []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefTarget0}, {Kind: kop.SoBaseValid, F: s.fCre, Arg: bf}, {Kind: kop.SoUnion}}
		}),
		// 6: the controllers of every creature, seat order from the controller
		6: mk(kop.OrderSeat, true, func(s *setTbl) []kop.SetOp {
			return []kop.SetOp{{Kind: kop.SoBaseValid, F: s.fAnyC, Arg: bf}, {Kind: kop.SoController}}
		}),
		// 7: the players who are not the controller
		7: mk(kop.OrderSeat, true, func(*setTbl) []kop.SetOp {
			return []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefController, Arg: 1}, {Kind: kop.SoComplementPlayers}}
		}),
		// 8: every creature, grouped by controller from the controller
		8: mk(kop.OrderSeat, false, func(s *setTbl) []kop.SetOp {
			return []kop.SetOp{{Kind: kop.SoBaseValid, F: s.fAnyC, Arg: bf}}
		}),
		// 9: each player (a player base)
		9: mk(kop.OrderList, true, func(*setTbl) []kop.SetOp { return []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefEachPlayer, Arg: 1}} }),
		// 10: the owners of the creatures
		10: mk(kop.OrderObj, true, func(s *setTbl) []kop.SetOp {
			return []kop.SetOp{{Kind: kop.SoBaseValid, F: s.fAnyC, Arg: bf}, {Kind: kop.SoOwner}}
		}),
	}
	st.t.Sets = sets
	p := newSetPos(t, st)
	s, k := p.sc.s, p.sc.k
	eval := func(id kop.SetID, ctrl uint8, t0 Obj) []Obj {
		var out setVal
		s.evalSet(k, id, p.ctxFor(ctrl, t0), &out)
		return members(&out)
	}
	for _, c := range []struct {
		name string
		id   kop.SetID
		ctrl uint8
		t0   Obj
		want []Obj
	}{
		{"target 0", 1, 0, p.c, []Obj{p.c}},
		{"creatures you control, seat 0", 2, 0, p.a, []Obj{p.a, p.b}},
		{"creatures you control, seat 1", 2, 1, p.a, []Obj{p.c}},
		{"inter", 3, 0, p.a, []Obj{p.a, p.b, p.c}},
		{"union, list order", 4, 0, p.c, []Obj{p.c, p.a, p.b}},
		{"union, object order", 5, 0, p.c, []Obj{p.a, p.b, p.c}},
		{"union dedupes", 4, 0, p.a, []Obj{p.a, p.b}},
		{"controllers from seat 0", 6, 0, p.a, []Obj{0, 1}},
		{"controllers from seat 1", 6, 1, p.a, []Obj{1, 0}},
		{"complement of the controller", 7, 0, p.a, []Obj{1}},
		{"complement of seat 1", 7, 1, p.a, []Obj{0}},
		{"creatures by controller from seat 1", 8, 1, p.a, []Obj{p.c, p.a, p.b}},
		{"each player from seat 1", 9, 1, p.a, []Obj{1, 0}},
		{"owners", 10, 1, p.a, []Obj{0, 1}},
	} {
		if got := eval(c.id, c.ctrl, c.t0); !slices.Equal(got, c.want) {
			t.Errorf("%s: members %v, want %v", c.name, got, c.want)
		}
	}
	for _, c := range []struct{ id kop.SetID }{{6}, {7}, {9}, {10}} {
		var out setVal
		s.evalSet(k, c.id, p.ctxFor(0, p.a), &out)
		if !out.isPl {
			t.Errorf("set %d: not a players set", c.id)
		}
	}
}

// TestEvalSetLKI: a creature that left the battlefield is read as it last was:
// SoLKI before SoController gives the controller it left under, not its owner.
func TestEvalSetLKI(t *testing.T) {
	st := newSetTbl(t, []kop.SetExpr{
		{Ops: []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefTarget0}, {Kind: kop.SoController}}, Players: true},
		{Ops: []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefTarget0}, {Kind: kop.SoLKI}, {Kind: kop.SoController}}, Players: true},
	})
	p := newSetPos(t, st)
	s, k := p.sc.s, p.sc.k
	// a creature seat 1 owns that seat 0 controls
	o := p.sc.obj(v.GrizzlyBears, 1)
	sl := s.enter(k, o, 0, false, kop.ZoneNone)
	s.npend = 0
	if s.perms[sl].ctrl != 0 || s.objs[o].owner() != 1 {
		t.Fatalf("precondition: controller %d owner %d, want 0 and 1", s.perms[sl].ctrl, s.objs[o].owner())
	}
	read := func(id kop.SetID) []Obj {
		var out setVal
		s.evalSet(k, id, p.ctxFor(0, o), &out)
		return members(&out)
	}
	if got := read(2); !slices.Equal(got, []Obj{0}) {
		t.Fatalf("on the battlefield, SoLKI controller %v, want [0]", got)
	}
	s.leave(k, sl, kop.ZoneGraveyard, false)
	if s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatalf("precondition: the creature is in zone %d, want the graveyard", s.objs[o].zone)
	}
	if got := read(1); !slices.Equal(got, []Obj{1}) {
		t.Errorf("without SoLKI, the card off the battlefield reads its owner: %v, want [1]", got)
	}
	if got := read(2); !slices.Equal(got, []Obj{0}) {
		t.Errorf("with SoLKI, the card reads the controller it left under: %v, want [0]", got)
	}
}

// TestEvalSetSetRefFlags: SrOncePerController keeps each controller's first
// member, in the set's order.
func TestEvalSetSetRefFlags(t *testing.T) {
	st := newSetTbl(t, nil)
	st.t.Sets = []kop.SetExpr{{}, {Ops: []kop.SetOp{{Kind: kop.SoBaseValid, F: st.fAnyC, Arg: int16(kop.ZoneBattlefield)}}, Order: kop.OrderSeat}}
	p := newSetPos(t, st)
	s, k := p.sc.s, p.sc.k
	var out setVal
	s.evalSetAs(k, 1, p.ctxFor(0, p.a), kop.SrOncePerController, &out)
	if got := members(&out); !slices.Equal(got, []Obj{p.a, p.c}) {
		t.Errorf("once per controller: %v, want [%d %d]", got, p.a, p.c)
	}
	s.evalSetAs(k, 1, p.ctxFor(1, p.a), kop.SrOncePerController, &out)
	if got := members(&out); !slices.Equal(got, []Obj{p.c, p.a}) {
		t.Errorf("once per controller from seat 1: %v, want [%d %d]", got, p.c, p.a)
	}
}

// TestSetOverflowStalls: a set past setMax members is StallSetOverflow.
func TestSetOverflowStalls(t *testing.T) {
	st := newSetTbl(t, nil)
	p := newSetPos(t, st)
	s := p.sc.s
	var val setVal
	val.reset(false, false)
	for i := 0; i < setMax; i++ {
		s.setPush(&val, Obj(i))
	}
	if s.Stalled() || val.n != setMax {
		t.Fatalf("a full set: stalled %v, %d members, want none stalled and %d", s.Stalled(), val.n, setMax)
	}
	s.setPush(&val, Obj(setMax))
	if s.StallReason() != StallSetOverflow || val.n != setMax {
		t.Fatalf("one past: stall %d (%s), %d members; want StallSetOverflow and %d", s.StallReason(), StallName(s.StallReason()), val.n, setMax)
	}
}

// TestSetEvalNoAlloc: a full program (bases, inter, union, controller) allocates
// nothing; gate step 3's evidence at the unit level.
func TestSetEvalNoAlloc(t *testing.T) {
	bf := int16(kop.ZoneBattlefield)
	st := newSetTbl(t, nil)
	st.t.Sets = []kop.SetExpr{{}, {Ops: []kop.SetOp{
		{Kind: kop.SoBaseValid, F: st.fCre, Arg: bf},
		{Kind: kop.SoBaseRef, R: kop.RefTarget0},
		{Kind: kop.SoUnion},
		{Kind: kop.SoBaseValid, Arg: bf},
		{Kind: kop.SoInter, F: st.fAnyC},
		{Kind: kop.SoUnion},
		{Kind: kop.SoLKI},
		{Kind: kop.SoController},
		{Kind: kop.SoBaseRef, R: kop.RefController, Arg: 1},
		{Kind: kop.SoUnion},
	}, Players: true, Order: kop.OrderSeat}}
	p := newSetPos(t, st)
	s, k := p.sc.s, p.sc.k
	ctx := p.ctxFor(0, p.c)
	var out setVal
	s.evalSet(k, 1, ctx, &out)
	if got := members(&out); !slices.Equal(got, []Obj{0, 1}) {
		t.Fatalf("precondition: the program's members %v, want [0 1]", got)
	}
	if n := testing.AllocsPerRun(1000, func() { s.evalSet(k, 1, ctx, &out) }); n != 0 {
		t.Fatalf("evalSet allocates %v per call, want 0", n)
	}
}

// setSpell is a sorcery with three creature target slots whose program is prog;
// sets is the table's sets and refs the ability's SetRefs.
func setSpell(name string, fCre kop.FilterID, prog []kop.Instr, refs []kop.SetRef) kop.CardDef {
	var tg [kop.MaxTargets]kop.TargetSpec
	for i := 0; i < 3; i++ {
		tg[i] = kop.TargetSpec{Kind: kop.TgtObject, Filter: fCre}
	}
	return kop.CardDef{Name: name, Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Targets: tg, NTargets: 3, Prog: prog, SetRefs: refs}}}
}

// castSet casts the sorcery spell over targets and resolves it, answering every
// counter-order ask with its first option; asks is how many it posed. mid, when
// not nil, runs at the first ask.
func (sc *scen) castSet(t *testing.T, spell Obj, targets []Obj, mid func()) (asks int) {
	t.Helper()
	sc.castCard(t, spell)
	ti := 0
	for i := 0; i < 200; i++ {
		d := sc.s.Step()
		if sc.s.over {
			return asks // a stall ended the game
		}
		if d.Kind == DkPriority {
			if sc.s.sp == 0 && sc.s.npend == 0 {
				return asks
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		if sc.s.res.ask == raGenCtrOrder {
			asks++
			if asks == 1 && mid != nil {
				mid()
			}
		}
		done := false
		for j := 0; j < int(d.N) && ti < len(targets) && sc.s.res.ask != raGenCtrOrder; j++ {
			if a := d.Options[j]; a.Kind() == AkTarget && a.Target() == TCode(targets[ti]) {
				sc.s.Apply(a)
				ti++
				done = true
				break
			}
		}
		if !done {
			sc.s.Apply(d.Options[0])
		}
	}
	t.Fatal("the spell never finished")
	return
}

func (sc *scen) ctrP1P1(o Obj) int { return sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kop.CtrP1P1)) }

// putPair builds two tables over the same cards: the set one, whose spell puts
// two +1/+1 counters on every target (one RefSet instruction over the targets),
// and the single one, the same spell with one single-object instruction per
// target. repl adds Hardened Scales and a doubler, so each placement asks.
func putPair(t *testing.T, repl bool) (set, single *setTbl) {
	t.Helper()
	build := func(useSet bool) *setTbl {
		st := newSetTbl(t, []kop.SetExpr{{Ops: []kop.SetOp{{Kind: kop.SoBaseRef, R: kop.RefTargets012}}, Order: kop.OrderObj}})
		var prog []kop.Instr
		var refs []kop.SetRef
		if useSet {
			prog = []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefSet, A: kop.Const(2), C: int16(kop.CtrP1P1)}}
			refs = []kop.SetRef{{PC: 0, Set: 1}}
		} else {
			for _, r := range []kop.Ref{kop.RefTarget0, kop.RefTarget1, kop.RefTarget2} {
				prog = append(prog, kop.Instr{Op: kop.OpPutCounter, Obj: r, A: kop.Const(2), C: int16(kop.CtrP1P1)})
			}
			// a SetRef no instruction reads: it only makes the table a generated
			// one, so its replacements apply as they do in the set table
			refs = []kop.SetRef{{PC: 0, Set: 1}}
		}
		cards := []kop.CardDef{setSpell("Test Put Set", st.fAnyC, prog, refs)}
		if repl {
			for _, rk := range []struct {
				kind kop.ReplKind
				a    int16
				n    string
			}{{kop.ReplDoubleCountersOn, 0, "Test Branching"}, {kop.ReplAddCountersOn, 1, "Test Scales"}} {
				cards = append(cards, kop.CardDef{Name: rk.n, Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbReplacement,
					Repl: kop.Repl{Kind: rk.kind, C: int16(kop.CtrP1P1), A: rk.a, Filter: st.fCre}}}})
			}
		}
		first := len(st.t.Cards)
		st.t.Cards = append(append([]kop.CardDef(nil), st.t.Cards...), cards...)
		for i := range cards {
			st.cards = append(st.cards, CardID(first+i))
		}
		return st
	}
	return build(true), build(false)
}

// putRun runs the spell of st over three seat-0 creatures and returns the
// position, the creatures and the asks the order competition posed.
func putRun(t *testing.T, st *setTbl, repl bool, mid func(*scen)) (*scen, [3]Obj, int) {
	t.Helper()
	sc := newScenTable(t, st.t, Config{NoElide: true})
	var cr [3]Obj
	for i := range cr {
		cr[i] = sc.perm(v.GrizzlyBears, 0)
	}
	if repl {
		sc.perm(st.cards[1], 0)
		sc.perm(st.cards[2], 0)
	}
	spell := sc.hand(st.cards[0], 0)
	asks := sc.castSet(t, spell, cr[:], func() {
		if mid != nil {
			mid(sc)
		}
	})
	return sc, cr, asks
}

// TestForSetPutCounterMatchesSingle: the same counters put through RefSet over
// the three targets and through three single-object instructions leave equal
// projections, without and with the order-asking replacements.
func TestForSetPutCounterMatchesSingle(t *testing.T) {
	for _, repl := range []bool{false, true} {
		set, single := putPair(t, repl)
		a, ca, asksA := putRun(t, set, repl, nil)
		b, cb, asksB := putRun(t, single, repl, nil)
		for i := range ca {
			if a.ctrP1P1(ca[i]) == 0 {
				t.Fatalf("repl %v: member %d got no counters through the set", repl, i)
			}
			if a.ctrP1P1(ca[i]) != b.ctrP1P1(cb[i]) {
				t.Errorf("repl %v: member %d has %d counters through the set, %d through the single instruction", repl, i, a.ctrP1P1(ca[i]), b.ctrP1P1(cb[i]))
			}
		}
		if repl && (asksA != 3 || asksB != 3) {
			t.Errorf("order asks: set %d, single %d, want 3 each (one per placement)", asksA, asksB)
		}
		if !repl && (asksA != 0 || asksB != 0) {
			t.Errorf("order asks with no replacements: set %d, single %d, want 0", asksA, asksB)
		}
		var pa, pb Projection
		a.s.Project(&pa)
		b.s.Project(&pb)
		if !reflect.DeepEqual(pa, pb) {
			t.Errorf("repl %v: the projections differ between the set and the single-object spells", repl)
		}
	}
}

// TestForSetSuspendResume: a RefSet put whose placements pose the CR 616.1 order
// ask suspends on each member and resumes where it stopped: every member gets
// its counters once, in order. A copy of the state taken at the first
// suspension and resumed the same way ends in the same projection.
func TestForSetSuspendResume(t *testing.T) {
	set, single := putPair(t, true)
	if !tableHasSet(set.t, true) {
		t.Fatal("precondition: the table has no suspension-capable RefSet op")
	}
	var cp *scen
	sc, cr, asks := putRun(t, set, true, func(sc *scen) {
		it := sc.s.setIterOf()
		if it == nil || !it.live || it.cur != 0 || it.skip {
			t.Fatalf("at the first ask: iterator %+v, want live on member 0, resume at the member", it)
		}
		// a copy of the state mid-suspension
		c := new(State)
		c.CopyFrom(sc.s)
		cp = &scen{s: c, k: c.k()}
	})
	if asks != 3 {
		t.Fatalf("order asks %d, want 3 (members 1 to 3 each pose one)", asks)
	}
	ref, cr2, _ := putRun(t, single, true, nil)
	for i := range cr {
		if sc.ctrP1P1(cr[i]) != ref.ctrP1P1(cr2[i]) || sc.ctrP1P1(cr[i]) == 0 {
			t.Errorf("member %d has %d counters, the single-object spell gave %d (once each, not 0)", i, sc.ctrP1P1(cr[i]), ref.ctrP1P1(cr2[i]))
		}
	}
	if it := sc.s.setIterOf(); it == nil || it.live {
		t.Errorf("iterator still live after the spell finished: %+v", it)
	}
	// finish the copy: the same answers from the point it was taken
	cp.s.copyDrive(t)
	for i := range cr {
		if cp.ctrP1P1(cr[i]) != sc.ctrP1P1(cr[i]) {
			t.Errorf("copy: member %d has %d counters, the original run %d", i, cp.ctrP1P1(cr[i]), sc.ctrP1P1(cr[i]))
		}
	}
	var pa, pb Projection
	sc.s.Project(&pa)
	cp.s.Project(&pb)
	if !reflect.DeepEqual(pa, pb) {
		t.Error("resuming on the mid-suspension copy ends in a different projection")
	}
}

// copyDrive finishes the resolution in progress on s: it answers each
// counter-order ask with its first option and passes until the stack is empty.
func (s *State) copyDrive(t *testing.T) {
	t.Helper()
	for i := 0; i < 200; i++ {
		d := s.Step()
		if d.Kind == DkPriority {
			if s.sp == 0 && s.npend == 0 {
				return
			}
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		s.Apply(d.Options[0])
	}
	t.Fatal("the copy never finished")
}

// TestSetIterSecondIterator: a RefSet instruction that starts while another
// iteration is frozen is StallSetIter, a declared stop, not a corruption.
func TestSetIterSecondIterator(t *testing.T) {
	set, _ := putPair(t, true)
	var sc *scen
	var spell Obj
	st := set
	sc = newScenTable(t, st.t, Config{NoElide: true})
	for i := 0; i < 3; i++ {
		sc.perm(v.GrizzlyBears, 0)
	}
	sc.perm(st.cards[1], 0)
	sc.perm(st.cards[2], 0)
	spell = sc.hand(st.cards[0], 0)
	asked := false
	sc.castSet(t, spell, nil, func() {
		asked = true
		it := sc.s.setIterOf()
		it.pc++ // another instruction owns the live iterator
	})
	if !asked {
		t.Fatal("precondition: no order ask was posed")
	}
	if sc.s.StallReason() != StallSetIter {
		t.Fatalf("stall %d (%s), want StallSetIter", sc.s.StallReason(), StallName(sc.s.StallReason()))
	}
}

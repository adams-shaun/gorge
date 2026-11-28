package dirac

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// setFCreature is vanilla's creature filter (vanilla fCreature).
const setFCreature kop.FilterID = 1

// setScen is an n-seat vanilla game with a resolving ability of a Grizzly
// Bears (self) controlled by ctrl: target 0 an opponent's Savannah Lions,
// target 1 the next seat, the trigger object an opponent's Plains and the
// trigger player that seat.
type setScen struct {
	*scen
	self, lions, plains Obj
	opp                 uint8
}

func newSetScen(t *testing.T, n, ctrl uint8) *setScen {
	t.Helper()
	tbl := *vanilla.Table
	if tbl.Filters[setFCreature].Types != kop.TCreature {
		t.Fatal("precondition: vanilla filter 1 is the creature filter")
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = vanilla.Plains
	}
	var decks [MaxSeats][]CardID
	for p := range decks {
		decks[p] = lib[:]
	}
	s := new(State)
	s.InitN(&tbl, decks, n, 1, Config{NoElide: true})
	sc := &setScen{scen: &scen{s: s, k: s.k()}}
	sc.opp = (ctrl + 1) % n
	sc.self = sc.perm(vanilla.GrizzlyBears, ctrl)
	sc.lions = sc.perm(vanilla.SavannahLions, sc.opp)
	sc.plains = sc.perm(vanilla.Plains, sc.opp)
	e := stackEntry{kind: seAbility, obj: sc.self, srcVer: s.objs[sc.self].ver, card: vanilla.GrizzlyBears, ctrl: ctrl, trigObj: sc.plains, trigPl: sc.opp}
	for i := range e.tgt {
		e.tgt[i] = NoTarget
	}
	e.tgt[0], e.tver[0] = TCode(sc.lions), s.objs[sc.lions].ver
	e.tgt[1] = PlayerCode(sc.opp)
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	return sc
}

// eval evaluates prog in the resolving entry's context and lists its members
// in walk order.
func (sc *setScen) eval(prog ...kop.SetOp) []TCode {
	s := sc.s
	v := s.evalSetExpr(sc.k, prog, setCtx{ev: s.ctxEntry(&s.res.e)})
	var list [MaxObj + MaxSeats]TCode
	var vers [MaxObj + MaxSeats]uint8
	n := s.setMembers(&v, s.res.e.ctrl, &list, &vers)
	return slices.Clone(list[:n])
}

func setOp1(k kop.SetOpKind, arg uint16) kop.SetOp { return kop.SetOp{Kind: k, Arg: arg} }

func setWant(t *testing.T, name string, got []TCode, want ...TCode) {
	t.Helper()
	if want == nil {
		want = []TCode{}
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func setPanics(t *testing.T, name, wantMsg string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if r == nil {
			t.Errorf("%s did not panic", name)
			return
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, wantMsg) {
			t.Errorf("%s panicked %q, want it to name %q", name, msg, wantMsg)
		}
	}()
	f()
}

// TestEvalSetBases: each push-base kind names what its gorge Defined$
// referent names, and the declared memory bases refuse by name.
func TestEvalSetBases(t *testing.T) {
	sc := newSetScen(t, 2, 0)
	if sc.self == sc.lions || sc.lions == sc.plains || sc.opp != 1 {
		t.Fatal("precondition: three distinct objects and an opponent at seat 1")
	}
	L, S, P, me, op := TCode(sc.lions), TCode(sc.self), TCode(sc.plains), PlayerCode(0), PlayerCode(1)
	setWant(t, "SetTarget 0", sc.eval(setOp1(kop.SetTarget, 0)), L)
	setWant(t, "SetTarget 1", sc.eval(setOp1(kop.SetTarget, 1)), op)
	setWant(t, "SetTarget 2 (empty slot)", sc.eval(setOp1(kop.SetTarget, 2)))
	setWant(t, "SetTargets 0|1", sc.eval(setOp1(kop.SetTargets, 3)), L, op)
	setWant(t, "SetTargets 1|0 keeps slot order", sc.eval(setOp1(kop.SetTargets, 2|1)), L, op)
	setWant(t, "SetSelf", sc.eval(setOp1(kop.SetSelf, 0)), S)
	setWant(t, "SetController", sc.eval(setOp1(kop.SetController, 0)), me)
	setWant(t, "SetTriggerObj", sc.eval(setOp1(kop.SetTriggerObj, 0)), P)
	setWant(t, "SetTriggerSource", sc.eval(setOp1(kop.SetTriggerSource, 0)), P)
	setWant(t, "SetTriggerPlayer", sc.eval(setOp1(kop.SetTriggerPlayer, 0)), op)
	setWant(t, "SetEachOpponent", sc.eval(setOp1(kop.SetEachOpponent, 0)), op)
	setWant(t, "SetEachPlayer", sc.eval(setOp1(kop.SetEachPlayer, 0)), me, op)

	// a target illegal on resolution is left out (CR 608.2b)
	sc.s.res.bad |= 1
	setWant(t, "SetTarget 0 once illegal", sc.eval(setOp1(kop.SetTarget, 0)))
	sc.s.res.bad = 0

	for _, k := range []kop.SetOpKind{kop.SetRemembered, kop.SetImprinted, kop.SetExiledWith, kop.SetChosen, kop.SetAttached, kop.SetSacrificed, kop.SetDiscarded} {
		setPanics(t, k.String(), k.String(), func() { sc.eval(setOp1(k, 0)) })
	}
}

// TestEvalSetCombinators: the filter keeps a target base's order and drops its
// players; a union walks in object index order then seat order; the
// complement, controller-of and owner-of read the players they name.
func TestEvalSetCombinators(t *testing.T) {
	sc := newSetScen(t, 2, 0)
	s := sc.s
	L, S, P, me, op := TCode(sc.lions), TCode(sc.self), TCode(sc.plains), PlayerCode(0), PlayerCode(1)
	if !(S < L && L < P) {
		t.Fatal("precondition: the objects were made self, lions, plains")
	}
	and := setOp1(kop.SetAndFilter, uint16(setFCreature))
	union := setOp1(kop.SetUnion, 0)
	setWant(t, "TriggerObj ∩ creature", sc.eval(setOp1(kop.SetTriggerObj, 0), and))
	setWant(t, "Targets ∩ creature", sc.eval(setOp1(kop.SetTargets, 3), and), L)
	setWant(t, "(Self ∪ TriggerObj) ∩ creature", sc.eval(setOp1(kop.SetSelf, 0), setOp1(kop.SetTriggerObj, 0), union, and), S)
	setWant(t, "TriggerObj ∪ Self ∪ Opponent ∪ You", sc.eval(setOp1(kop.SetTriggerObj, 0), setOp1(kop.SetSelf, 0), union,
		setOp1(kop.SetEachOpponent, 0), union, setOp1(kop.SetController, 0), union), S, P, me, op)
	setWant(t, "Targets ∪ Self is index order", sc.eval(setOp1(kop.SetTargets, 3), setOp1(kop.SetSelf, 0), union), S, L, op)
	setWant(t, "Self ∪ Self is one member", sc.eval(setOp1(kop.SetSelf, 0), setOp1(kop.SetSelf, 0), union), S)
	setWant(t, "Non You", sc.eval(setOp1(kop.SetController, 0), setOp1(kop.SetPlayersComplement, 0)), op)
	setWant(t, "Non Player", sc.eval(setOp1(kop.SetEachPlayer, 0), setOp1(kop.SetPlayersComplement, 0)))

	// a controlled-not-owned permanent: controller-of and owner-of differ
	s.perms[s.objs[sc.lions].slot].ctrl = 0
	setWant(t, "ControllerOf Target 0", sc.eval(setOp1(kop.SetTarget, 0), setOp1(kop.SetControllerOf, 0)), me)
	setWant(t, "OwnerOf Target 0", sc.eval(setOp1(kop.SetTarget, 0), setOp1(kop.SetOwnerOf, 0)), op)
	setWant(t, "ControllerOf Targets keeps the player target", sc.eval(setOp1(kop.SetTargets, 3), setOp1(kop.SetControllerOf, 0)), me, op)
	setWant(t, "OwnerOf Self", sc.eval(setOp1(kop.SetSelf, 0), setOp1(kop.SetOwnerOf, 0)), me)

	for _, k := range []kop.SetOpKind{kop.SetLKIOf, kop.SetSpawnerOf} {
		setPanics(t, k.String(), k.String(), func() { sc.eval(setOp1(kop.SetSelf, 0), setOp1(k, 0)) })
	}
	setPanics(t, "a union of one set", "SetUnion", func() { sc.eval(setOp1(kop.SetSelf, 0), union) })
	setPanics(t, "two sets left", "leaves 2", func() { sc.eval(setOp1(kop.SetSelf, 0), setOp1(kop.SetSelf, 0)) })
	if d, ok := (kop.SetExpr{setOp1(kop.SetSelf, 0), setOp1(kop.SetSelf, 0), union, and}).SetDepth(); !ok || d != 2 {
		t.Errorf("SetDepth = %d, %v; want 2, true", d, ok)
	}
}

// TestEvalSetOrderFourSeats: at four seats the players walk in turn order from
// the controller (gorge AliveFrom), from each controller, and a seat that has
// left the game is no player or opponent.
func TestEvalSetOrderFourSeats(t *testing.T) {
	for ctrl := uint8(0); ctrl < 4; ctrl++ {
		sc := newSetScen(t, 4, ctrl)
		if sc.s.nSeats() != 4 {
			t.Fatal("precondition: a four-seat game")
		}
		var all, opps []TCode
		for i := uint8(0); i < 4; i++ {
			p := (ctrl + i) % 4
			all = append(all, PlayerCode(p))
			if p != ctrl {
				opps = append(opps, PlayerCode(p))
			}
		}
		name := fmt.Sprintf("ctrl %d ", ctrl)
		setWant(t, name+"EachPlayer", sc.eval(setOp1(kop.SetEachPlayer, 0)), all...)
		setWant(t, name+"EachOpponent", sc.eval(setOp1(kop.SetEachOpponent, 0)), opps...)
		setWant(t, name+"Non You", sc.eval(setOp1(kop.SetController, 0), setOp1(kop.SetPlayersComplement, 0)), opps...)
		// objects first, then the seats from the controller
		setWant(t, name+"Self ∪ EachPlayer", sc.eval(setOp1(kop.SetEachPlayer, 0), setOp1(kop.SetSelf, 0), setOp1(kop.SetUnion, 0)),
			append([]TCode{TCode(sc.self)}, all...)...)

		gone := (ctrl + 2) % 4
		sc.s.pl[gone].lost = true
		if !sc.s.out(gone) {
			t.Fatal("precondition: the seat is out of the game")
		}
		var left []TCode
		for _, c := range all {
			if c != PlayerCode(gone) {
				left = append(left, c)
			}
		}
		setWant(t, name+"EachPlayer with a seat out", sc.eval(setOp1(kop.SetEachPlayer, 0)), left...)
		setWant(t, name+"Non You with a seat out", sc.eval(setOp1(kop.SetController, 0), setOp1(kop.SetPlayersComplement, 0)), left[1:]...)
	}
}

// TestEvalSetDoesNotAllocate: evaluating a set and listing its members runs on
// the Go stack.
func TestEvalSetDoesNotAllocate(t *testing.T) {
	sc := newSetScen(t, 4, 1)
	s := sc.s
	prog := kop.SetExpr{
		setOp1(kop.SetTargets, 3), setOp1(kop.SetSelf, 0), setOp1(kop.SetUnion, 0),
		setOp1(kop.SetAndFilter, uint16(setFCreature)),
		setOp1(kop.SetEachPlayer, 0), setOp1(kop.SetUnion, 0),
		setOp1(kop.SetTriggerObj, 0), setOp1(kop.SetControllerOf, 0), setOp1(kop.SetUnion, 0),
	}
	sc.k.t.Sets = []kop.SetExpr{prog}
	ab := &kop.Ability{SetRefs: []kop.SetRef{{PC: 3, Set: 0}}}
	var list [MaxObj + MaxSeats]TCode
	var vers [MaxObj + MaxSeats]uint8
	n := 0
	allocs := testing.AllocsPerRun(200, func() {
		v := s.evalSet(sc.k, ab, 3, setCtx{ev: s.ctxEntry(&s.res.e)})
		n = s.setMembers(&v, s.res.e.ctrl, &list, &vers)
	})
	if n != 2+4 {
		t.Fatalf("precondition: the set has %d members, want the two creatures and four players", n)
	}
	if allocs != 0 {
		t.Errorf("evalSet allocates %.1f times per run", allocs)
	}
}

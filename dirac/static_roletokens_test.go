package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const (
	kroleBear CardID = 0
	kroleRole CardID = 1
	kroleRock CardID = 2
)

// kroleTable is a hand-written table with a creature, an Aura Role token
// (Enchant: creature, "when this leaves the battlefield, its controller
// loses 1 life") and a noncreature artifact.
func kroleTable() *kop.Table {
	return &kop.Table{
		Cards: []kop.CardDef{
			{Name: "Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			{Name: "Role", Types: kop.TEnchantment | kop.TAura,
				Subtypes: kop.SubMask{}.With(0),
				Enchant:  kop.TargetSpec{Kind: kop.TgtObject, Filter: kop.FilterID(1)},
				Abilities: []kop.Ability{{
					Kind: kop.AbTriggered,
					Trig: kop.Trigger{Event: kop.TrigLeaves, Flags: kop.TfSelf},
					Prog: []kop.Instr{{Op: kop.OpLoseLife, Obj: kop.RefController, A: kop.Const(1)}},
				}}},
			{Name: "Rock", Types: kop.TArtifact},
		},
		Filters:  []kop.Filter{{}, {Types: kop.TCreature}},
		Subtypes: []string{"Role"},
		Conds:    []kop.Cond{},
	}
}

func kroleScen(t *testing.T) *scen {
	t.Helper()
	return newScenTableLib(t, kroleTable(), kroleBear, Config{})
}

// kroleMint runs createAttachedToken with dst in target slot 0 and returns
// the token's object and slot.
func kroleMint(t *testing.T, sc *scen, owner uint8, dst Obj) (Obj, Slot) {
	t.Helper()
	s := sc.s
	s.res.e.tgt[0] = TCode(dst)
	in := kop.Instr{Op: kop.OpCreateTokenEx, C: int16(kroleRole), A: kop.Const(1),
		B: kop.Const(int(kop.TokAttach) | int(kop.TokAttachN(kop.RefTarget0)))}
	if ok, done := s.createAttachedToken(sc.k, &in, owner, 1); !ok || !done {
		t.Fatalf("createAttachedToken = %v, %v; want true, true", ok, done)
	}
	o := s.res.lastTok
	return o, sc.slot(o)
}

// TestRoleAttachOwnTarget: a Role minted with AttachedTo$ Targeted enters
// attached to that permanent.
func TestRoleAttachOwnTarget(t *testing.T) {
	sc := kroleScen(t)
	bear := sc.perm(kroleBear, 0)
	o, sl := kroleMint(t, sc, 0, bear)
	if !sc.onBF(o) {
		t.Fatal("the Role is not on the battlefield")
	}
	if a := sc.s.perms[sl].attached; a != sc.slot(bear) {
		t.Fatalf("Role attached to slot %d, want %d", a, sc.slot(bear))
	}
	if !sc.s.hasSubGen(sc.k, sl, sc.k.gen.roleSub) {
		t.Fatal("the minted Role lost its Role subtype")
	}
}

// TestRoleWithheldNoBearer: a destination that left the battlefield means
// the Role is not created at all (CR 303.4g).
func TestRoleWithheldNoBearer(t *testing.T) {
	sc := kroleScen(t)
	before := sc.s.nobj
	s := sc.s
	s.res.e.tgt[0] = TCode(NoTarget)
	in := kop.Instr{Op: kop.OpCreateTokenEx, C: int16(kroleRole), A: kop.Const(1),
		B: kop.Const(int(kop.TokAttach) | int(kop.TokAttachN(kop.RefTarget0)))}
	if ok, done := s.createAttachedToken(sc.k, &in, 0, 1); !ok || !done {
		t.Fatalf("createAttachedToken = %v, %v; want true, true", ok, done)
	}
	if sc.s.nobj != before {
		t.Fatalf("nobj %d, want %d: a token was created", sc.s.nobj, before)
	}
}

// TestRoleGateNoCandidate: no permanent on the battlefield can be
// enchanted, so the Role is withheld (gorge's aura gate).
func TestRoleGateNoCandidate(t *testing.T) {
	sc := kroleScen(t)
	rock := sc.perm(kroleRock, 0)
	before := sc.s.nobj
	s := sc.s
	s.res.e.obj = rock
	in := kop.Instr{Op: kop.OpCreateTokenEx, C: int16(kroleRole), A: kop.Const(1),
		B: kop.Const(int(kop.TokAttach) | int(kop.TokAttachN(kop.RefSelf)))}
	if ok, done := s.createAttachedToken(sc.k, &in, 0, 1); !ok || !done {
		t.Fatalf("createAttachedToken = %v, %v; want true, true", ok, done)
	}
	if sc.s.nobj != before {
		t.Fatalf("nobj %d, want %d: a token was created", sc.s.nobj, before)
	}
}

// TestRoleSweepSameController: a second Role on the same bearer puts the
// first into its owner's graveyard.
func TestRoleSweepSameController(t *testing.T) {
	sc := kroleScen(t)
	bear := sc.perm(kroleBear, 0)
	first, _ := kroleMint(t, sc, 0, bear)
	second, sl := kroleMint(t, sc, 0, bear)
	sc.s.sba(sc.k)
	if sc.onBF(first) {
		t.Fatalf("the old Role is in zone %d, want the graveyard", sc.s.objs[first].zone)
	}
	if !sc.onBF(second) || sc.s.perms[sl].attached != sc.slot(bear) {
		t.Fatal("the new Role is not attached to the bear")
	}
}

// TestRoleCrossControllerCoexist: Roles of different controllers on one
// bearer both stay (CR 704.5z).
func TestRoleCrossControllerCoexist(t *testing.T) {
	sc := kroleScen(t)
	bear := sc.perm(kroleBear, 1)
	first, _ := kroleMint(t, sc, 0, bear)
	second, sl := kroleMint(t, sc, 1, bear)
	sc.s.sba(sc.k)
	if !sc.onBF(first) || !sc.onBF(second) || sc.s.perms[sl].attached != sc.slot(bear) {
		t.Fatal("Roles of different controllers did not coexist")
	}
}

// TestRoleWickedSweepTrigger: the swept Role's leave trigger fires (a plain
// move to the graveyard, not a destroy).
func TestRoleWickedSweepTrigger(t *testing.T) {
	sc := kroleScen(t)
	bear := sc.perm(kroleBear, 0)
	first, _ := kroleMint(t, sc, 0, bear)
	pend := sc.s.npend
	kroleMint(t, sc, 0, bear)
	sc.s.sba(sc.k)
	if sc.onBF(first) {
		t.Fatal("the old Role did not leave")
	}
	if sc.s.npend <= pend {
		t.Fatalf("pending triggers %d, want more than %d: the leave trigger did not fire", sc.s.npend, pend)
	}
}

// TestRoleAtMostOneOld: the sweep walks every Role on the bearer, so a board
// with two (forced) old Roles loses both when a third arrives.
func TestRoleAtMostOneOld(t *testing.T) {
	sc := kroleScen(t)
	bear := sc.perm(kroleBear, 0)
	host := sc.slot(bear)
	var old []Obj
	for i := 0; i < 2; i++ {
		o := sc.perm(kroleRole, 0)
		sc.s.perms[sc.slot(o)].attached = host
		old = append(old, o)
	}
	fresh := sc.perm(kroleRole, 0)
	sc.s.g.roleOut = 1 // forced Roles bypass createAttachedToken, which sets it
	sc.s.perms[sc.slot(fresh)].attached = host
	sc.s.lstamp[sc.slot(fresh)] = 99
	sc.s.sba(sc.k)
	for _, o := range old {
		if sc.s.objs[o].zone != kop.ZoneGraveyard {
			t.Fatalf("old Role %d is in zone %d, want the graveyard", o, sc.s.objs[o].zone)
		}
	}
	if !sc.onBF(fresh) {
		t.Fatal("the sweep removed the new Role")
	}
}

// TestRoleFlagRearm: roleOut clears once no attached Role remains, and a
// later mint sets it again so the next same-controller pair is swept.
func TestRoleFlagRearm(t *testing.T) {
	sc := kroleScen(t)
	bear := sc.perm(kroleBear, 0)
	r1, sl1 := kroleMint(t, sc, 0, bear)
	sc.s.leave(sc.k, sl1, kop.ZoneGraveyard, false)
	sc.s.sba(sc.k)
	if sc.s.g.roleOut != 0 {
		t.Fatal("roleOut did not clear with no Role on the battlefield")
	}
	r2, _ := kroleMint(t, sc, 0, bear)
	r3, _ := kroleMint(t, sc, 0, bear)
	sc.s.sba(sc.k)
	if sc.onBF(r1) || sc.onBF(r2) || !sc.onBF(r3) {
		t.Fatal("the re-armed sweep did not keep only the newest Role")
	}
}

// TestRoleSweepGroupedPerBearer: losers leave grouped per bearer, bearers in
// order of their first Role in the walk (gorge roleSBA). Walk order is
// X1, Y1, X2, Y2 with Y1 and X2 the losers; X2 leaves before Y1 because
// bearer X's first Role precedes bearer Y's.
func TestRoleSweepGroupedPerBearer(t *testing.T) {
	sc := kroleScen(t)
	bx := sc.perm(kroleBear, 0)
	by := sc.perm(kroleBear, 0)
	x1, sx1 := kroleMint(t, sc, 0, bx)
	y1, sy1 := kroleMint(t, sc, 0, by)
	x2, sx2 := kroleMint(t, sc, 0, bx)
	y2, sy2 := kroleMint(t, sc, 0, by)
	s := sc.s
	s.lstamp[sx1], s.lstamp[sy2] = 200, 200
	s.lstamp[sy1], s.lstamp[sx2] = 100, 100
	pend := int(s.npend)
	s.sba(sc.k)
	if !sc.onBF(x1) || !sc.onBF(y2) || sc.onBF(x2) || sc.onBF(y1) {
		t.Fatal("wrong Roles swept")
	}
	if int(s.npend)-pend != 2 {
		t.Fatalf("pending triggers +%d, want 2", int(s.npend)-pend)
	}
	if s.pend[pend].trigObj != x2 && s.pend[pend].obj != x2 {
		t.Fatalf("first leave trigger is not for X's loser: %+v", s.pend[pend])
	}
}

// TestRoleTieKeepsLaterInWalk: equal layer timestamps (a synthetic board)
// keep the Role later in the battlefield walk, not the higher slot (gorge
// roleSBA's >= over its AliveFrom(0) zone walk).
func TestRoleTieKeepsLaterInWalk(t *testing.T) {
	sc := kroleScen(t)
	bear := sc.perm(kroleBear, 0)
	a, sa := kroleMint(t, sc, 0, bear)
	b, sb := kroleMint(t, sc, 0, bear)
	s := sc.s
	lo, hi, olo, ohi := sa, sb, a, b
	if lo > hi {
		lo, hi, olo, ohi = hi, lo, ohi, olo
	}
	// the higher slot walks first; the lower slot is later in the walk
	s.perms[hi].ts, s.perms[lo].ts = 10, 20
	s.lstamp[sa], s.lstamp[sb] = 100, 100
	s.sba(sc.k)
	if !sc.onBF(olo) || sc.onBF(ohi) {
		t.Fatal("the tie did not keep the Role later in the walk")
	}
}

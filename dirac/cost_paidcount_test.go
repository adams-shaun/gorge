package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// paidCountTable is the vanilla table plus one card per repeated or conspired
// cost and the expression Count$SquadPaid / ReplicatePaid / Conspired lowers to
// (ExKicked N 2), returning the card ids and the expression's value ref.
func paidCountTable() (t *kop.Table, replicate, squad, conspire CardID, read kop.ValueRef) {
	t = v.NewTable()
	t.Exprs = append(t.Exprs, kop.Expr{Kind: kop.ExKicked, N: 2})
	read = kop.ValueRef{K: kop.VExpr, N: int16(len(t.Exprs) - 1)}
	add := func(c kop.CardDef) CardID {
		t.Cards = append(t.Cards, c)
		return CardID(len(t.Cards) - 1)
	}
	replicate = add(kop.CardDef{Name: "Test Replicate", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 1},
		Multikicker: kop.ManaCost{Generic: 1}, HasMultikicker: true, RepeatedKind: kop.RepeatReplicate})
	squad = add(kop.CardDef{Name: "Test Squad", Types: kop.TCreature, Power: 1, Toughness: 1, Cost: kop.ManaCost{Generic: 1},
		Multikicker: kop.ManaCost{Generic: 1}, HasMultikicker: true, RepeatedKind: kop.RepeatSquad,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigETB, Flags: kop.TfSelf}}}})
	conspire = add(kop.CardDef{Name: "Test Conspire", Types: kop.TSorcery, Cost: kop.ManaCost{Generic: 1}, Colors: kop.ColR, Conspire: true})
	return
}

// paidSpell makes card c's object a stack-zone spell entry of player 0.
func (sc *scen) paidSpell(c CardID) (stackEntry, Obj) {
	o := sc.obj(c, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	return stackEntry{kind: seSpell, obj: o, card: c, ctrl: 0}, o
}

// TestPaidCountReplicate: a Replicate spell's payments are stored per object
// (coldPaidN), read by Count$ReplicatePaid on the spell and on its own cast
// trigger (the source is the spell on the stack), cleared when the card is
// cast again, and allocated only for a table that has the keyword.
func TestPaidCountReplicate(t *testing.T) {
	if sc := newScenTable(t, v.NewTable(), Config{NoElide: true}); coldPaidN.of(sc.s) != nil {
		t.Fatal("a table with no Squad, Replicate or Conspire card allocated coldPaidN")
	}
	tbl, rep, _, _, read := paidCountTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if len(coldPaidN.of(s)) != MaxObj {
		t.Fatalf("coldPaidN has %d entries, want MaxObj %d", len(coldPaidN.of(s)), MaxObj)
	}
	e, o := sc.paidSpell(rep)
	if got := s.evalV(k, read, s.ctxEntry(&e)); got != 0 {
		t.Fatalf("unpaid replicate reads %d, want 0", got)
	}
	s.setMultikick(&e, 3)
	if got := coldPaidN.of(s)[o]; got != 3 {
		t.Fatalf("coldPaidN[obj] = %d after three payments, want 3", got)
	}
	if got := s.evalV(k, read, s.ctxEntry(&e)); got != 3 {
		t.Fatalf("the spell reads %d, want 3", got)
	}
	// the cast trigger's evaluation: a trigger entry whose source is the spell
	trig := stackEntry{kind: seTrigger, obj: o, card: rep, ctrl: 0}
	if got := s.evalV(k, read, s.ctxEntry(&trig)); got != 3 {
		t.Fatalf("the cast trigger reads %d, want 3", got)
	}
	// the count is the stored byte, not the build's: zeroing mkObj leaves it
	s.cold.mkObj[o] = 0
	if got := s.evalV(k, read, s.ctxEntry(&e)); got != 3 {
		t.Fatalf("the read follows cold.mkObj (%d), want coldPaidN's 3", got)
	}
	s.beginCast(k, o, 0, 0) // a card cast again starts with no payments
	if got := coldPaidN.of(s)[o]; got != 0 {
		t.Fatalf("coldPaidN[obj] = %d after a new cast began, want 0", got)
	}
}

// TestPaidCountSquad: a Squad creature's payments are read by its own ETB
// trigger from the count captured as the trigger was queued (the creature can
// leave first), and by the permanent through its object's byte; a new object
// reusing the id reads 0.
func TestPaidCountSquad(t *testing.T) {
	tbl, _, squad, _, read := paidCountTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	e, o := sc.paidSpell(squad)
	s.setMultikick(&e, 2)
	if got := s.evalV(k, read, s.ctxEntry(&e)); got != 2 {
		t.Fatalf("the squad spell reads %d, want 2", got)
	}
	// the permanent it became
	s.objs[o].zone = kop.ZoneBattlefield
	trig := stackEntry{kind: seTrigger, obj: o, card: squad, ctrl: 0, ab: 0, defSeat: 2}
	if got := s.evalV(k, read, s.ctxEntry(&trig)); got != 2 {
		t.Fatalf("the ETB trigger reads %d, want its captured 2", got)
	}
	// the creature left (Wasteland Raider's own sacrifice): the capture stays
	s.objs[o].zone = kop.ZoneGraveyard
	if got := s.evalV(k, read, s.ctxEntry(&trig)); got != 2 {
		t.Fatalf("the ETB trigger of a departed squad creature reads %d, want its captured 2", got)
	}
	// a token copy never cast reads 0: the id's reuse ends the old count
	s.objs[o].zone = kop.ZoneBattlefield
	s.reuseObj(o)
	if got := coldPaidN.of(s)[o]; got != 0 {
		t.Fatalf("coldPaidN[obj] = %d after the id was reused, want 0", got)
	}
	trig.defSeat = 0
	if got := s.evalV(k, read, s.ctxEntry(&trig)); got != 0 {
		t.Fatalf("a never-cast permanent reads %d, want 0", got)
	}
}

// TestPaidCountConspire: the paid conspire cast stores 1 as the spell goes on
// the stack; an unpaid or plain cast stores 0 (CR 702.78a, 603.4: the cast
// trigger's intervening "if" reads it). The conspired cast is offered from the
// printed keyword alone, beside the plain cast, when two creatures share a
// colour with the spell, and casting it taps both.
func TestPaidCountConspire(t *testing.T) {
	tbl, _, _, con, read := paidCountTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if k.gen == nil || !k.gen.anyConspire {
		t.Fatal("a printed Conspire card did not open the conspire gate")
	}
	e, o := sc.paidSpell(con)
	*s.bld() = build{e: e}
	s.bld().cas = casConspire // unpaid: the election has not been settled
	s.notePaidConspire(k, &e)
	if got := s.evalV(k, read, s.ctxEntry(&e)); got != 0 {
		t.Fatalf("an unpaid conspire reads %d, want 0", got)
	}
	s.bld().cas = casConspPaid
	s.notePaidConspire(k, &e)
	if got := s.evalV(k, read, s.ctxEntry(&e)); got != 1 {
		t.Fatalf("a paid conspire reads %d, want 1", got)
	}
	if got := coldPaidN.of(s)[o]; got != 1 {
		t.Fatalf("coldPaidN[obj] = %d, want 1", got)
	}

	// the cast flow: offered beside the plain cast, taps the two creatures
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	hand := sc2.hand(con, 0)
	a, b := sc2.perm(v.RagingGoblin, 0), sc2.perm(v.RagingGoblin, 0)
	sc2.perm(v.Mountain, 0)
	sc2.perm(v.Mountain, 0)
	d := sc2.s.Step()
	plain, conspired := false, false
	for i := 0; i < int(d.N); i++ {
		switch d.Options[i].Kind() {
		case AkCast:
			plain = plain || d.Options[i].Source() == TCode(hand)
		case AkCastConspire:
			conspired = conspired || d.Options[i].Source() == TCode(hand)
		}
	}
	if !plain || !conspired {
		t.Fatalf("plain cast offered %v, conspired cast offered %v, want both", plain, conspired)
	}
	sc2.do(t, AkCastConspire, TCode(hand), scAny)
	for i := 0; i < 12 && sc2.s.sp == 0; i++ {
		d = sc2.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc2.s.Apply(d.Options[0])
	}
	if sc2.s.sp != 1 {
		t.Fatalf("the conspired spell is not on the stack (sp=%d)", sc2.s.sp)
	}
	for _, o := range []Obj{a, b} {
		if sc2.s.perms[sc2.slot(o)].flags&pfTapped == 0 {
			t.Fatalf("creature %d was not tapped for conspire", o)
		}
	}
	top := sc2.s.stack[0]
	if got := sc2.s.evalV(sc2.k, read, sc2.s.ctxEntry(&top)); got != 1 {
		t.Fatalf("the conspired spell on the stack reads %d, want 1", got)
	}

	// the plain cast of the same spell stores 0
	sc3 := newScenTable(t, tbl, Config{NoElide: true})
	hand3 := sc3.hand(con, 0)
	sc3.perm(v.RagingGoblin, 0)
	sc3.perm(v.RagingGoblin, 0)
	sc3.perm(v.Mountain, 0)
	sc3.perm(v.Mountain, 0)
	sc3.do(t, AkCast, TCode(hand3), scAny)
	for i := 0; i < 12 && sc3.s.sp == 0; i++ {
		d = sc3.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc3.s.Apply(d.Options[0])
	}
	if sc3.s.sp != 1 {
		t.Fatalf("the plain spell is not on the stack (sp=%d)", sc3.s.sp)
	}
	top = sc3.s.stack[0]
	if got := sc3.s.evalV(sc3.k, read, sc3.s.ctxEntry(&top)); got != 0 {
		t.Fatalf("the plain cast reads %d, want 0", got)
	}
}

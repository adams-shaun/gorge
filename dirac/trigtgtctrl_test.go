package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// trigtgtctrlTable builds a costless sorcery whose body is one dig of the
// top card into exile with the dig player RefTriggerTgtCtrl: the dig window
// of gorge's Defined$ TriggeredTargetController (Expedited Inheritance's
// "its controller may exile that many cards").
func trigtgtctrlTable() (*kop.Table, kop.CardID) {
	tbl := v.NewTable()
	// a non-nil expression table makes prepareGen build the generated
	// extension the OpDigEx op needs
	tbl.Exprs = []kop.Expr{}
	c := kop.CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Triggered Target Controller Dig",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpDigEx, Obj: kop.RefTriggerTgtCtrl, A: kop.Const(1), B: kop.Const(-1),
				C: int16(kop.ZoneExile)},
		}}},
	})
	return tbl, c
}

// trigtgtctrlRun casts the spell with the resolving trigger object bound to
// its referent (the per-referent queue's trigObj binding; trigPl keeps the
// stale queue-time player 0) and returns the object the dig exiled: seat
// 0's top card is the myr and seat 1's is the token, so the exiled card
// names the library the ref resolved to; the return is that seat. The
// referent is seat 1's myr on seat 0's battlefield, dead (in seat 0's
// graveyard) when dead is set.
func trigtgtctrlRun(t *testing.T, dead bool) uint8 {
	t.Helper()
	tbl, c := trigtgtctrlTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	// a triggerless creature stands in for the damaged referent (Perilous
	// Myr's dies trigger would pose its own target decision)
	referent := sc.perm(v.GrizzlyBears, 0)
	s.objs[referent].setOwner(1)
	if dead {
		s.leave(sc.k, s.objs[referent].slot, kop.ZoneGraveyard, false)
	}
	myr := sc.obj(v.Island, 0)
	tok := sc.obj(v.SoldierToken, 0)
	s.pl[0].libTop(myr)
	s.pl[1].libTop(tok)

	o := sc.hand(c, 0)
	sc.do(t, AkCast, TCode(o), scAny)
	sc.do(t, AkPass, scAny, scAny)
	// the resolving spell stands in for the queued trigger: bind its
	// trigger object and the stale queue-time trigger player
	for i := range s.stack {
		if s.stack[i].card == c {
			s.stack[i].kind, s.stack[i].trigObj, s.stack[i].trigPl = seTrigger, referent, 0
		}
	}
	sc.do(t, AkPass, scAny, scAny)
	for {
		d := s.Step()
		if d.Kind == DkPriority {
			break
		}
		if d.Kind != DkNone && d.Kind != DkChoose {
			t.Fatalf("unexpected decision %v", d)
		}
	}
	if s.objs[tok].zone == kop.ZoneExile {
		return 1
	}
	if s.objs[myr].zone == kop.ZoneExile {
		return 0
	}
	t.Fatalf("neither top card was exiled: myr in zone %d, token in zone %d", s.objs[myr].zone, s.objs[tok].zone)
	return 255
}

// TestTriggerTgtCtrlReadsLive: kop.RefTriggerTgtCtrl (gorge Defined$
// TriggeredTargetController) reads the triggering referent's controller
// live at resolution. A referent that left the battlefield since it was
// dealt the damage reads its owner (CR 603.7a: Expedited Inheritance's dig
// window is the dead creature's owner's library, never the queue-time
// controller's); a surviving referent reads its current controller, which
// is the value the queue-time binding carried.
func TestTriggerTgtCtrlReadsLive(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	s := sc.s

	// a referent owned by seat 1, controlled by seat 0 on the battlefield
	live := sc.perm(v.PerilousMyr, 0)
	s.objs[live].setOwner(1)

	// both resolver homes agree on a surviving referent: its live controller
	s.res.e = stackEntry{kind: seTrigger, trigObj: live, trigPl: 0, ctrl: 0}
	if got := s.playerRef(sc.k, kop.RefTriggerTgtCtrl); got != 0 {
		t.Fatalf("playerRef on a live referent: %d, want its controller 0", got)
	}
	if got := s.refPlayer(sc.k, kop.RefTriggerTgtCtrl, s.ctxEntry(&s.res.e)); got != 0 {
		t.Fatalf("refPlayer on a live referent: %d, want its controller 0", got)
	}

	// the referent dies from the damage: the read is its owner (both homes)
	s.leave(sc.k, s.objs[live].slot, kop.ZoneGraveyard, false)
	if got := s.playerRef(sc.k, kop.RefTriggerTgtCtrl); got != 1 {
		t.Fatalf("playerRef on a dead referent: %d, want its owner 1", got)
	}
	if got := s.refPlayer(sc.k, kop.RefTriggerTgtCtrl, s.ctxEntry(&s.res.e)); got != 1 {
		t.Fatalf("refPlayer on a dead referent: %d, want its owner 1", got)
	}

	// no referent bound: the queue-time trigger player stands in
	s.res.e.trigObj = NoObj
	if got := s.playerRef(sc.k, kop.RefTriggerTgtCtrl); got != 0 {
		t.Fatalf("playerRef with no referent: %d, want the trigger player 0", got)
	}
}

// TestTriggerTgtCtrlDigWindow: the dig instruction carrying the ref resolves
// its window to the referent's controller as it exists at resolution: the
// dead referent's owner's library (seat 1's), a surviving referent's
// controller's (seat 0's).
func TestTriggerTgtCtrlDigWindow(t *testing.T) {
	if got := trigtgtctrlRun(t, true); got != 1 {
		t.Fatalf("dig took from seat %d, want seat 1 (the dead referent's owner's window)", got)
	}
	if got := trigtgtctrlRun(t, false); got != 0 {
		t.Fatalf("dig took from seat %d, want seat 0 (the live referent's controller's window)", got)
	}
}

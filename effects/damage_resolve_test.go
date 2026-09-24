package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// batchSpyHost wraps the effects test double and counts the damage-batch
// brackets the dealDamage loops open, plus the damage amount emitted inside
// each bracket. It lets an effects-level test pin that a marked flush is ONE
// batch without extending the shared fakeHost (the rules-level batch latch is
// pinned separately). Emit and EmitDamage are both overridden because
// emitObjectDamage/emitPlayerDamage reach the log through EmitDamage, which on
// the embedded double calls its own concrete Emit.
type batchSpyHost struct {
	*fakeHost
	opens int
	deals []int32
	cur   int32
}

func (h *batchSpyHost) BeginDamageBatch() { h.opens++; h.cur = 0 }
func (h *batchSpyHost) EndDamageBatch()   { h.deals = append(h.deals, h.cur) }

func (h *batchSpyHost) Emit(e events.Event) {
	if e.Kind == events.Damage {
		h.cur += e.Amount
	}
	h.fakeHost.Emit(e)
}

func (h *batchSpyHost) EmitDamage(e events.Event) events.Event {
	if e.Kind == events.Damage {
		h.cur += e.Amount
	}
	return h.fakeHost.EmitDamage(e)
}

// damageCarrier is a one-card fixture whose DamageResolve tail the tests drive.
const damageCarrier = "Name:Spy Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"

// lifelinkCarrier is a damage SOURCE with lifelink, so the flush's rider can
// be observed as a LifeChange for its controller.
const lifelinkCarrier = "Name:Spy Lifelinker\nTypes:Creature\nPT:1/1\nK:Lifelink\nOracle:x\n"

func battlefield(t *testing.T, h *fakeHost, src string) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, src), 0)
	o.Zone = state.ZBattlefield
	return o.ID
}

func countDamage(log []events.Event) int {
	n := 0
	for _, ev := range log {
		if ev.Kind == events.Damage {
			n++
		}
	}
	return n
}

// TestDealDamageDamageMapDefersUntilResolve pins the mark half of the
// primitive: a DamageMap$ True DealDamage records its damage on the resolving
// Ctx and emits NOTHING (no Damage event, no life change), so a bare
// DamageResolve later deals it. It also asserts the precondition the assertion
// depends on -- the damage source is on the battlefield (so the rider reads a
// live controller) and the Ctx started with no pending marks.
func TestDealDamageDamageMapDefersUntilResolve(t *testing.T) {
	h := newHost(t, 2)
	src := battlefield(t, h, lifelinkCarrier)
	h.g.Players[1].Life = 20
	if h.g.Obj(src).Zone != state.ZBattlefield {
		t.Fatalf("precondition: source zone = %s, want battlefield", h.g.Obj(src).Zone)
	}
	c := &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	if len(c.PendingDamage) != 0 {
		t.Fatalf("precondition: Ctx starts with %d pending marks, want 0", len(c.PendingDamage))
	}

	// The marking call: DealDamage with DamageMap$ True.
	Resolve(h, c, sa(t, "SP$ DealDamage | Defined$ Player.Opponent | NumDmg$ 2 | DamageMap$ True"))
	if n := countDamage(h.log); n != 0 {
		t.Fatalf("marked DealDamage emitted %d Damage events, want 0 (the damage must defer)", n)
	}
	if h.g.Players[1].Life != 20 {
		t.Fatalf("life after the marking call = %d, want 20 (nothing dealt yet)", h.g.Players[1].Life)
	}
	if len(c.PendingDamage) != 1 {
		t.Fatalf("pending marks = %d, want 1", len(c.PendingDamage))
	}
	if h.g.Players[0].Life != 20 {
		t.Fatalf("lifelink must not pay before the flush: seat 0 life = %d, want 20", h.g.Players[0].Life)
	}

	// The flush: DB$ DamageResolve.
	Resolve(h, c, sa(t, "DB$ DamageResolve"))
	if n := countDamage(h.log); n != 1 {
		t.Fatalf("flush emitted %d Damage events, want exactly 1", n)
	}
	if h.g.Players[1].Life != 18 {
		t.Fatalf("life after the flush = %d, want 18", h.g.Players[1].Life)
	}
	if len(c.PendingDamage) != 0 {
		t.Fatalf("pending marks after the flush = %d, want 0 (consumed)", len(c.PendingDamage))
	}
	// The lifelink rider pays EXACTLY ONCE, from the source's mark-time facts.
	if h.g.Players[0].Life != 22 {
		t.Fatalf("lifelink life after the flush = %d, want 22 (paid once, +2)", h.g.Players[0].Life)
	}
}

// TestDamageResolveFlushIsOneBatch pins the batch half: two marked calls flush
// inside ONE BeginDamageBatch/EndDamageBatch bracket whose total is the sum of
// the marks, and the marking calls themselves open NO bracket. That is the
// observable the primitive exists for -- a DamageDealtOnce trigger latches once
// with the batch total instead of once per marked call.
func TestDamageResolveFlushIsOneBatch(t *testing.T) {
	spy := &batchSpyHost{fakeHost: newHost(t, 2)}
	spy.g.Players[1].Life = 20
	first := &Ctx{Source: battlefield(t, spy.fakeHost, damageCarrier), Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	// A second mark on the same recipient, the Lunge/Cunning Strike shape: two
	// calls, one flush.
	Resolve(spy, first, sa(t, "SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2 | DamageMap$ True | Defined$ Player.Opponent"))
	Resolve(spy, first, sa(t, "DB$ DealDamage | Defined$ Player.Opponent | NumDmg$ 2 | DamageMap$ True"))
	if len(first.PendingDamage) != 2 {
		t.Fatalf("precondition: pending marks = %d, want 2", len(first.PendingDamage))
	}
	if spy.opens != 0 {
		t.Fatalf("the marking calls opened %d damage batches, want 0 (no damage dealt yet)", spy.opens)
	}

	Resolve(spy, first, sa(t, "DB$ DamageResolve"))
	if spy.opens != 1 {
		t.Fatalf("the flush opened %d damage batches, want exactly 1", spy.opens)
	}
	if len(spy.deals) != 1 || spy.deals[0] != 4 {
		t.Fatalf("batch totals = %v, want [4] (one batch of 2+2)", spy.deals)
	}
	if spy.g.Players[1].Life != 16 {
		t.Fatalf("life after the flush = %d, want 16", spy.g.Players[1].Life)
	}
}

// TestDamageResolveWithNoMarksIsSilent is the "nothing happens" test that must
// ALSO prove the handler ran: a bare DamageResolve with no marks must emit
// nothing at all -- in particular no "unimplemented API DamageResolve" Note.
// With the registration reverted to the unregistered fallback, the Note lands
// and this fails; it cannot pass with the feature unregistered.
func TestDamageResolveWithNoMarksIsSilent(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Source: 0, Controller: 0}
	if len(c.PendingDamage) != 0 {
		t.Fatalf("precondition: Ctx starts with %d pending marks, want 0", len(c.PendingDamage))
	}
	Resolve(h, c, sa(t, "DB$ DamageResolve"))
	if len(h.log) != 0 {
		t.Fatalf("a zero-mark DamageResolve emitted %d events, want 0 (silent no-op): %+v", len(h.log), h.log)
	}
	if len(c.PendingDamage) != 0 {
		t.Fatalf("pending marks after a bare flush = %d, want 0", len(c.PendingDamage))
	}
}

// TestDamageResolveFlushWalksItsSubAbility pins that a flush is an ordinary
// chain link: a SubAbility$ after a DamageResolve still runs (Cunning Strike's
// DBDraw shape). It also pins deferral across the chain walk.
func TestDamageResolveFlushWalksItsSubAbility(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[1].Life = 20
	var ran bool
	Register("TestDamageResolveTail", func(Host, *Ctx, *cards.SA) { ran = true })
	t.Cleanup(func() { unregister("TestDamageResolveTail") })

	src := "Name:T\nTypes:Sorcery\n" +
		"A:SP$ DealDamage | Defined$ Player.Opponent | NumDmg$ 2 | DamageMap$ True | SubAbility$ R\n" +
		"SVar:R:DB$ DamageResolve | SubAbility$ Tail\n" +
		"SVar:Tail:DB$ TestDamageResolveTail\nOracle:x\n"
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	ctx := &Ctx{Controller: 0}
	Resolve(h, ctx, c.Faces[0].Abilities[0])
	if !ran {
		t.Fatal("the SubAbility after the flush did not run")
	}
	if h.g.Players[1].Life != 18 {
		t.Fatalf("life = %d, want 18 (the flush dealt the marked 2)", h.g.Players[1].Life)
	}
}

package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestSlowMotionPaymentPreventsSacrifice uses Slow Motion's real compiled
// SacrificeAll SA. Its EnchantedController payer is offered the may-pay
// decision, and the rules-side paid re-entry prevents the sacrifice.
func TestSlowMotionPaymentPreventsSacrifice(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	slow, ok := reg.Lookup("Slow Motion")
	if !ok {
		t.Fatal("Slow Motion missing from corpus")
	}
	bear := mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	h := &askHost{fakeHost: fakeHost{g: state.NewGame(names(2))}}
	src := h.g.AddObject(slow, 0).ID
	victim := h.g.AddObject(bear, 1).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: victim, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.Attach, Obj: src, IDs: []state.ObjID{victim}})
	sa := cards.ResolveSVar(slow.Faces[0].SVars, "TrigUpkeep")
	if sa == nil || sa.API != "SacrificeAll" {
		t.Fatalf("Slow Motion TrigUpkeep = %#v, want SacrificeAll", sa)
	}
	ctx := &Ctx{Source: src, Controller: 0}
	Resolve(h, ctx, sa)
	if h.asked == nil || h.asked.ResumeKind != "unless_pay" || h.asked.Player != 1 {
		t.Fatalf("Slow Motion payer decision = %+v, want seat 1 unless_pay", h.asked)
	}
	ctx.UnlessPay = "pay" // rules resumeResolution sets this after payMana.
	Resolve(h, ctx, sa)
	if got := h.g.Obj(victim).Zone; got != state.ZBattlefield {
		t.Fatalf("paid Slow Motion sacrificed creature into %s", got)
	}
}

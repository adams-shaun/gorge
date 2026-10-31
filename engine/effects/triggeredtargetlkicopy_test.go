package effects

// ds2: Defined$ TriggeredTargetLKICopy must name the referent the CAUSING
// mode captured (CR 603.2, 608.2c), not one fixed role.
//
// The spelling used to answer Ctx.TriggerBearer when an Attached trigger set
// it and Ctx.Remembered otherwise. For a Mode$ BecomesTarget trigger
// Remembered is the TARGETING SPELL/ABILITY (rules' triggerReferents seeds it
// from ev.Obj), so Cloud Cover's "return that permanent to its owner's hand"
// and Cowardice's "return that creature" bounced the spell, and Horobi's and
// Fractured Loyalty's bodies named the spell instead of the permanent that
// became the target. The targeted permanent is the TriggerTarget role
// becomesTargetReferent captures.
//
// The three shapes below are the three the corpus pairs with this spelling
// (census in the report): BecomesTarget (9 files), Attached (1), and the
// DamageDone/DamageDoneOnce family (51). Each asserts its PRECONDITION (the
// roles differ, the objects are live) so a vacuous board cannot pass.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// lkiCopyBoard builds a two-seat board: a battlefield watcher (the trigger's
// source), a battlefield creature that becomes the target/bearer, a spell on
// the stack (what a BecomesTarget capture stores in Remembered), and an Aura
// on the battlefield. Every object is live, and the ids are distinct.
func lkiCopyBoard(t *testing.T) (h *fakeHost, ids map[string]state.ObjID) {
	t.Helper()
	h = newHost(t, 2)
	watcher := h.g.AddObject(mkCard(t, "Name:Watcher\nManaCost:1 U\nTypes:Enchantment\nOracle:x\n"), 0)
	perm := h.g.AddObject(mkCard(t, "Name:Target Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	spell := h.g.AddObject(mkCard(t, "Name:Zap\nManaCost:R\nTypes:Sorcery\nOracle:x\n"), 1)
	aura := h.g.AddObject(mkCard(t, "Name:Favor\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"), 0)
	h.g.Obj(perm.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{watcher.ID, perm.ID})
	h.g.Obj(aura.ID).Zone = state.ZBattlefield
	h.g.Obj(aura.ID).AttachedTo = perm.ID
	h.g.Obj(spell.ID).Zone = state.ZStack
	h.g.SetZone(state.ZStack, 1, []state.ObjID{spell.ID})
	ids = map[string]state.ObjID{
		"watcher": watcher.ID, "perm": perm.ID, "spell": spell.ID, "aura": aura.ID,
	}
	for k, id := range ids {
		if id == 0 || h.g.Obj(id) == nil {
			t.Fatalf("precondition: %s is not a live object", k)
		}
	}
	if ids["perm"] == ids["spell"] {
		t.Fatal("precondition: the targeted permanent and the targeting spell are the same object")
	}
	return h, ids
}

// wantDefined resolves one Defined$ spelling and requires it to name exactly
// the single live object want.
func wantDefined(t *testing.T, h *fakeHost, c *Ctx, spec string, want state.ObjID) {
	t.Helper()
	ts, ok := definedSpec(h, c, spec)
	if !ok {
		t.Fatalf("Defined$ %s: not recognised", spec)
	}
	if len(ts) != 1 {
		t.Fatalf("Defined$ %s = %+v, want exactly [{Obj: %d}]", spec, ts, want)
	}
	if ts[0].IsPlayer || ts[0].Obj != want {
		t.Fatalf("Defined$ %s = %+v, want the object %d", spec, ts, want)
	}
	if h.g.Obj(ts[0].Obj) == nil {
		t.Fatalf("Defined$ %s named %d, which is not a live object", spec, ts[0].Obj)
	}
}

// TestTriggeredTargetLKICopyBecomesTargetNamesTheTargetedPermanent is Cloud
// Cover/Cowardice/Horobi/Fractured Loyalty: the capture the BecomesTarget arm
// installs (TriggerStack = the targeting spell, TriggerTarget = the targeted
// permanent, Remembered = the targeting spell) must resolve to the permanent.
func TestTriggeredTargetLKICopyBecomesTargetNamesTheTargetedPermanent(t *testing.T) {
	h, ids := lkiCopyBoard(t)
	c := NewCtx(ids["watcher"], 0, CtxInit{
		Remembered: []state.Target{{Obj: ids["spell"]}},
		TriggerContext: TriggerContext{
			TriggerTarget: state.Target{Obj: ids["perm"]},
			TriggerStack:  ids["spell"],
		},
	})
	if c.TriggerBearer != 0 {
		t.Fatalf("precondition: a BecomesTarget capture sets no TriggerBearer (got %d)", c.TriggerBearer)
	}
	// The two roles this spelling can name must differ, or the test is vacuous.
	if c.TriggerTarget.Obj == c.Remembered[0].Obj {
		t.Fatalf("precondition: TriggerTarget and Remembered both name %d", c.TriggerTarget.Obj)
	}
	wantDefined(t, h, &c, "TriggeredTargetLKICopy", ids["perm"])
	// The plain spelling the TriggeredAttached-style bodies use must agree.
	wantDefined(t, h, &c, "TriggeredTarget", ids["perm"])
}

// TestTriggeredTargetLKICopyAttachedStillNamesTheBearer is Enormous Energy
// Blade's "tap that creature": the Attached arm keeps resolving the bearer,
// which it must even though that arm also fills TriggerTarget.
func TestTriggeredTargetLKICopyAttachedStillNamesTheBearer(t *testing.T) {
	h, ids := lkiCopyBoard(t)
	c := NewCtx(ids["aura"], 0, CtxInit{
		Remembered: []state.Target{{Obj: ids["aura"]}},
		TriggerContext: TriggerContext{
			TriggerTarget: state.Target{Obj: ids["perm"]},
			TriggerBearer: ids["perm"],
		},
	})
	if c.TriggerStack != 0 {
		t.Fatalf("precondition: an Attached capture sets no TriggerStack (got %d)", c.TriggerStack)
	}
	if c.TriggerBearer == 0 || c.TriggerBearer == c.Source {
		t.Fatalf("precondition: bearer %d must be a real permanent other than the source %d", c.TriggerBearer, c.Source)
	}
	wantDefined(t, h, &c, "TriggeredTargetLKICopy", c.TriggerBearer)
}

// TestTriggeredTargetLKICopyDamageTriggerKeepsRemembered pins the family that
// must NOT move: 51 corpus files pair this spelling with DamageDone/
// DamageDoneOnce, whose capture fills TriggerTarget without the targeting
// marker, and those resolve the Remembered object exactly as before.
func TestTriggeredTargetLKICopyDamageTriggerKeepsRemembered(t *testing.T) {
	h, ids := lkiCopyBoard(t)
	c := NewCtx(ids["watcher"], 0, CtxInit{
		Remembered: []state.Target{{Obj: ids["spell"]}},
		TriggerContext: TriggerContext{
			TriggerTarget: state.Target{Obj: ids["perm"]},
		},
	})
	if c.TriggerStack != 0 || c.TriggerBearer != 0 {
		t.Fatalf("precondition: a non-targeting capture sets neither marker (stack %d, bearer %d)",
			c.TriggerStack, c.TriggerBearer)
	}
	if c.TriggerTarget.Obj == c.Remembered[0].Obj {
		t.Fatalf("precondition: TriggerTarget and Remembered both name %d", c.TriggerTarget.Obj)
	}
	wantDefined(t, h, &c, "TriggeredTargetLKICopy", ids["spell"])
}

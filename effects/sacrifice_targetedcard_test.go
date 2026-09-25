package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestSacValidTargetedCardSelf is the focused candidate-pool regression: a
// player-targeted Sacrifice whose SacValid$ names the resolution's card target
// (`TargetedCard.Self`) must offer exactly that target -- and only when it sits
// in the asked player's battlefield. Enchanter's Bane's compiled DBSac is this
// shape. Before the fix the target-referent base was unrecognised by the filter
// grammar (a bare Self reads relative to the resolving SOURCE), so the pool was
// empty and the optional ask was never posed.
//
// Board: the targeted enchantment and an unrelated enchantment both belong to
// seat 1, so the pool has two candidates and the compared values (which option
// is offered) genuinely differ.
func TestSacValidTargetedCardSelf(t *testing.T) {
	h := newHost(t, 2)
	target := putBattlefield(h, 1, "Name:Target Enchantment\nTypes:Enchantment\nOracle:x\n")
	unrelated := putBattlefield(h, 1, "Name:Other Enchantment\nTypes:Enchantment\nOracle:x\n")
	// Precondition: both candidates are live battlefield permanents controlled
	// by the player the target names, and are distinct objects.
	if o := h.g.Obj(target); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: target = %+v; want a battlefield permanent controlled by seat 1", h.g.Obj(target))
	}
	if o := h.g.Obj(unrelated); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: unrelated = %+v; want a battlefield permanent controlled by seat 1", h.g.Obj(unrelated))
	}
	if target == unrelated {
		t.Fatalf("precondition: target and unrelated are the same id %d", target)
	}

	c := &Ctx{Source: 0, Controller: 0, Targets: []state.Target{{Obj: target}}}
	h.askResult = true
	effSacrifice(h, c, sacrificeParams(map[string]string{
		"Defined": "TargetedController", "SacValid": "TargetedCard.Self", "Optional": "True",
	}))

	// Precondition: the target resolves to seat 1 (its controller), so the ask
	// goes to the right seat. If Defined$ TargetedController stopped resolving,
	// the ask would go elsewhere and the option assertion below is meaningless.
	if h.lastAsk == nil {
		t.Fatal("no ask was posed; want a KChoose offering the targeted card")
	}
	if h.lastAsk.Kind != decision.KChoose || h.lastAsk.Player != 1 {
		t.Fatalf("ask = %+v; want a KChoose to seat 1", h.lastAsk)
	}
	if h.lastAsk.Min != 0 || h.lastAsk.Max != 1 {
		t.Fatalf("ask range = %d..%d; want 0..1 (Optional$ True)", h.lastAsk.Min, h.lastAsk.Max)
	}
	if len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != target {
		t.Fatalf("options = %+v; want only targeted card %d (unrelated %d excluded)", h.lastAsk.Options, target, unrelated)
	}
}

// TestEnchantersBaneOptionalSacrifice is the corpus-backed regression. The real
// compiled DBSac of Enchanter's Bane targets an enchantment and offers its
// controller the option to sacrifice it. The test first pins the compiled SA
// shape (API/Defined/SacValid/Optional), so it cannot silently stop exercising
// the card if a compile change alters it, then drives both answers through the
// exact resume the engine's KChoose handler produces (rules/resolution.go's
// "sacrifice" arm: SacPicks holds the chosen ids, empty means decline).
func TestEnchantersBaneOptionalSacrifice(t *testing.T) {
	card, ok := testutil.CorpusRegistry(t).Lookup("Enchanter's Bane")
	if !ok {
		t.Fatal("corpus has no Enchanter's Bane")
	}
	sac := cards.ResolveSVar(card.Faces[0].SVars, "DBSac")
	if sac == nil {
		t.Fatal("Enchanter's Bane has no compiled DBSac SVar")
	}
	if sac.API != "Sacrifice" || sac.Params["Defined"] != "TargetedController" ||
		sac.Params["SacValid"] != "TargetedCard.Self" || sac.Params["Optional"] != "True" {
		t.Fatalf("compiled sacrifice SA = API %q params %#v; want Sacrifice / TargetedController / TargetedCard.Self / True",
			sac.API, sac.Params)
	}

	for _, answer := range []string{"decline", "sacrifice"} {
		t.Run(answer, func(t *testing.T) {
			h := newHost(t, 2)
			// The Bane sits on seat 0; the targeted enchantment on seat 1, so
			// the target belongs to the player asked and is NOT the source.
			bane := h.g.AddObject(card, 0)
			bane.Zone = state.ZBattlefield
			h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), bane.ID))
			target := putBattlefield(h, 1, "Name:Target Enchantment\nTypes:Enchantment\nOracle:x\n")
			if o := h.g.Obj(bane.ID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
				t.Fatalf("precondition: Bane = %+v; want a battlefield permanent controlled by seat 0", h.g.Obj(bane.ID))
			}
			if o := h.g.Obj(target); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
				t.Fatalf("precondition: target = %+v; want a battlefield permanent controlled by seat 1", h.g.Obj(target))
			}
			if bane.ID == target {
				t.Fatalf("precondition: Bane and target are the same id %d", bane.ID)
			}

			c := &Ctx{Source: bane.ID, Controller: 0, Targets: []state.Target{{Obj: target}}}
			h.askResult = true
			effSacrifice(h, c, sac)

			if h.lastAsk == nil || h.lastAsk.Kind != decision.KChoose ||
				h.lastAsk.Player != 1 || h.lastAsk.Min != 0 || h.lastAsk.Max != 1 {
				t.Fatalf("choice = %+v; want target controller's Min 0 / Max 1 KChoose", h.lastAsk)
			}
			if len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != target {
				t.Fatalf("options = %+v; want targeted enchantment %d", h.lastAsk.Options, target)
			}

			// Feed the KChoose answer back exactly as the engine's resume arm
			// does: SacDone + SacTarget identify the answered target, SacPicks
			// is the chosen batch (empty = decline).
			c.SacDone = true
			c.SacTarget = 0
			if answer == "decline" {
				c.SacPicks = nil
			} else {
				c.SacPicks = []state.ObjID{target}
			}
			effSacrifice(h, c, sac)

			want := state.ZBattlefield
			if answer == "sacrifice" {
				want = state.ZGraveyard
			}
			if got := h.g.Obj(target).Zone; got != want {
				t.Fatalf("target zone = %v; want %v", got, want)
			}
		})
	}
}

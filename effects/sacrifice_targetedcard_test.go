package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestSacValidTargetedCardSelf(t *testing.T) {
	h := newHost(t, 2)
	target := putBattlefield(h, 1, "Name:Target Enchantment\nTypes:Enchantment\nOracle:x\n")
	unrelated := putBattlefield(h, 1, "Name:Other Enchantment\nTypes:Enchantment\nOracle:x\n")
	if h.g.Obj(target).Zone != state.ZBattlefield || h.g.Obj(unrelated).Zone != state.ZBattlefield || target == unrelated {
		t.Fatalf("precondition: target=%+v unrelated=%+v; want distinct battlefield objects", h.g.Obj(target), h.g.Obj(unrelated))
	}

	c := &Ctx{Source: 0, Controller: 0, Targets: []state.Target{{Obj: target}}}
	h.askResult = true
	effSacrifice(h, c, sacrificeParams(map[string]string{
		"Defined": "TargetedController", "SacValid": "TargetedCard.Self", "Optional": "True",
	}))
	if h.lastAsk == nil || h.lastAsk.Kind != decision.KChoose {
		t.Fatalf("ask = %+v, want KChoose", h.lastAsk)
	}
	if len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != target {
		t.Fatalf("options = %+v, want only targeted card %d (unrelated %d excluded)", h.lastAsk.Options, target, unrelated)
	}
}

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
		t.Fatalf("compiled sacrifice SA = API %q params %#v; want Sacrifice / TargetedController / TargetedCard.Self / True", sac.API, sac.Params)
	}

	for _, answer := range []string{"decline", "sacrifice"} {
		t.Run(answer, func(t *testing.T) {
			h := newHost(t, 2)
			bane := h.g.AddObject(card, 0)
			bane.Zone = state.ZBattlefield
			h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), bane.ID))
			target := putBattlefield(h, 1, "Name:Target Enchantment\nTypes:Enchantment\nOracle:x\n")
			if h.g.Obj(target).Zone != state.ZBattlefield || h.g.Obj(target).Controller != 1 || bane.ID == target {
				t.Fatalf("precondition: bane=%+v target=%+v; want distinct battlefield objects controlled by seats 0 and 1", h.g.Obj(bane.ID), h.g.Obj(target))
			}
			c := &Ctx{Source: bane.ID, Controller: 0, Targets: []state.Target{{Obj: target}}}
			h.askResult = true
			effSacrifice(h, c, sac)
			if h.lastAsk == nil || h.lastAsk.Kind != decision.KChoose || h.lastAsk.Player != 1 || h.lastAsk.Min != 0 || h.lastAsk.Max != 1 {
				t.Fatalf("choice = %+v, want target controller's Min 0 / Max 1 KChoose", h.lastAsk)
			}
			if len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != target {
				t.Fatalf("options = %+v, want targeted enchantment %d", h.lastAsk.Options, target)
			}

			c.SacDone = true
			c.SacTarget = 0
			if answer == "decline" {
				c.SacOptional = "decline"
				c.SacOptionalTarget = 0
			} else {
				c.SacPicks = []state.ObjID{target}
			}
			effSacrifice(h, c, sac)
			want := state.ZBattlefield
			if answer == "sacrifice" {
				want = state.ZGraveyard
			}
			if got := h.g.Obj(target).Zone; got != want {
				t.Fatalf("target zone = %v, want %v", got, want)
			}
		})
	}
}

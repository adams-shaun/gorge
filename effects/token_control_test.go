package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func putTokenControlPermanent(t *testing.T, h *fakeHost, owner state.PlayerID, types string) {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:Control probe\nTypes:"+types+"\nPT:2/2\nOracle:x\n"), owner)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, owner, append(h.g.Zone(state.ZBattlefield, owner), o.ID))
}

func tokenControlTokenCount(h *fakeHost, p state.PlayerID, name string) int {
	n := 0
	for _, id := range h.g.Zone(state.ZBattlefield, p) {
		o := h.g.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

func TestTokenOwnerPlayerControlQualifiers(t *testing.T) {
	t.Run("artifact-or-enchantment", func(t *testing.T) {
		h := newHost(t, 3)
		h.g.Tokens = map[string]*cards.Card{"bear": mkCard(t, "Name:Bear Token\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")}
		putTokenControlPermanent(t, h, 1, "Artifact")
		putTokenControlPermanent(t, h, 2, "Enchantment")
		Resolve(h, &Ctx{Controller: 0}, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{
			"TokenScript": "bear", "TokenOwner": "Player.controlsEnchantment,controlsArtifact",
		}})
		if tokenControlTokenCount(h, 0, "Bear Token") != 0 || tokenControlTokenCount(h, 1, "Bear Token") != 1 || tokenControlTokenCount(h, 2, "Bear Token") != 1 {
			t.Fatalf("artifact/enchantment owners: seats have %d, %d, %d bears", tokenControlTokenCount(h, 0, "Bear Token"), tokenControlTokenCount(h, 1, "Bear Token"), tokenControlTokenCount(h, 2, "Bear Token"))
		}
		for _, ev := range h.log {
			if ev.Kind == events.Note && strings.Contains(ev.Text, "unrecognized TokenOwner") {
				t.Fatalf("qualified owner fell back: %q", ev.Text)
			}
		}
	})

	t.Run("fewest-creatures", func(t *testing.T) {
		h := newHost(t, 3)
		h.g.Tokens = map[string]*cards.Card{"salamander": mkCard(t, "Name:Salamander Token\nTypes:Creature Salamander\nPT:2/2\nOracle:x\n")}
		putTokenControlPermanent(t, h, 0, "Creature Bear")
		putTokenControlPermanent(t, h, 1, "Creature Bear")
		putTokenControlPermanent(t, h, 2, "Creature Bear")
		putTokenControlPermanent(t, h, 2, "Creature Bear")
		c := &Ctx{Controller: 0, SVars: map[string]string{"X": "PlayerCountPlayers$LowestValid Creature.YouCtrl"}}
		Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{
			"TokenScript": "salamander", "TokenOwner": "Player.controlsCreature_EQX",
		}})
		if tokenControlTokenCount(h, 0, "Salamander Token") != 1 || tokenControlTokenCount(h, 1, "Salamander Token") != 1 || tokenControlTokenCount(h, 2, "Salamander Token") != 0 {
			t.Fatalf("fewest-creature owners: seats have %d, %d, %d salamanders", tokenControlTokenCount(h, 0, "Salamander Token"), tokenControlTokenCount(h, 1, "Salamander Token"), tokenControlTokenCount(h, 2, "Salamander Token"))
		}
	})
}

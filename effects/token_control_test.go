package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
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

func corpusTokenSA(t *testing.T, card *cards.Card, wantOwner string) (*cards.SA, map[string]string) {
	t.Helper()
	var find func(*cards.SA) *cards.SA
	find = func(s *cards.SA) *cards.SA {
		if s == nil {
			return nil
		}
		if s.API == "Token" && s.Params["TokenOwner"] == wantOwner {
			return s
		}
		return find(s.Sub)
	}
	for _, f := range card.Faces {
		for _, a := range f.Abilities {
			if s := find(a); s != nil {
				return s, f.SVars
			}
		}
		for _, tr := range f.Triggers {
			if s := find(tr.Effect); s != nil {
				return s, f.SVars
			}
		}
	}
	t.Fatalf("%q TokenOwner %q SA not found", card.Faces[0].Name, wantOwner)
	return nil, nil
}

func TestTokenOwnerPlayerControlQualifiers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	t.Run("artifact-or-enchantment", func(t *testing.T) {
		card, ok := reg.Lookup("Fade from History")
		if !ok {
			t.Fatal("Fade from History missing from corpus")
		}
		sa, _ := corpusTokenSA(t, card, "Player.controlsEnchantment,controlsArtifact")
		if got := sa.Params["TokenOwner"]; got != "Player.controlsEnchantment,controlsArtifact" {
			t.Fatalf("TokenOwner precondition = %q", got)
		}
		h := newHost(t, 3)
		h.g.Tokens = reg.Tokens
		putTokenControlPermanent(t, h, 1, "Artifact")
		putTokenControlPermanent(t, h, 2, "Enchantment")
		Resolve(h, &Ctx{Controller: 0}, sa)
		if tokenControlTokenCount(h, 0, "Bear Token") != 0 || tokenControlTokenCount(h, 1, "Bear Token") != 1 || tokenControlTokenCount(h, 2, "Bear Token") != 1 {
			t.Fatalf("artifact/enchantment owners: seats have %d, %d, %d bears", tokenControlTokenCount(h, 0, "Bear Token"), tokenControlTokenCount(h, 1, "Bear Token"), tokenControlTokenCount(h, 2, "Bear Token"))
		}
		assertNoTokenOwnerFallbackNote(t, h)
	})

	t.Run("fewest-creatures", func(t *testing.T) {
		card, ok := reg.Lookup("Gor Muldrak, Amphinologist")
		if !ok {
			t.Fatal("Gor Muldrak, Amphinologist missing from corpus")
		}
		var face *cards.Face
		for _, f := range card.Faces {
			if _, ok := f.SVars["TrigToken"]; ok {
				face = f
				break
			}
		}
		if face == nil {
			t.Fatal("Gor Muldrak TrigToken face missing")
		}
		sa := cards.ResolveSVar(face.SVars, "TrigToken")
		if sa == nil || sa.API != "Token" {
			t.Fatalf("TrigToken = %#v, want Token SA", sa)
		}
		if got := sa.Params["TokenOwner"]; got != "Player.controlsCreature_EQX" {
			t.Fatalf("TokenOwner precondition = %q", got)
		}
		if got := face.SVars["X"]; got != "PlayerCountPlayers$LowestValid Creature.YouCtrl" {
			t.Fatalf("X precondition = %q", got)
		}
		h := newHost(t, 3)
		h.g.Tokens = reg.Tokens
		putTokenControlPermanent(t, h, 0, "Creature Bear")
		putTokenControlPermanent(t, h, 1, "Creature Bear")
		putTokenControlPermanent(t, h, 2, "Creature Bear")
		putTokenControlPermanent(t, h, 2, "Creature Bear")
		Resolve(h, &Ctx{Controller: 0, SVars: face.SVars}, sa)
		if tokenControlTokenCount(h, 0, "Salamander Warrior Token") != 1 || tokenControlTokenCount(h, 1, "Salamander Warrior Token") != 1 || tokenControlTokenCount(h, 2, "Salamander Warrior Token") != 0 {
			t.Fatalf("fewest-creature owners: seats have %d, %d, %d salamanders", tokenControlTokenCount(h, 0, "Salamander Warrior Token"), tokenControlTokenCount(h, 1, "Salamander Warrior Token"), tokenControlTokenCount(h, 2, "Salamander Warrior Token"))
		}
		assertNoTokenOwnerFallbackNote(t, h)
	})
}

func assertNoTokenOwnerFallbackNote(t *testing.T, h *fakeHost) {
	t.Helper()
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unrecognized TokenOwner") {
			t.Fatalf("qualified owner fell back: %q", ev.Text)
		}
	}
}

package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// referentHost is a 3-seat game with a plain permanent owned by seat 1 and
// controlled by seat 0 (a stolen card), and one library card per seat.
func referentHost(t *testing.T) (*fakeHost, *Ctx, *state.Object) {
	t.Helper()
	h := newHost(t, 3)
	filler := mkCard(t, "Name:Filler\nTypes:Creature\nPT:1/1\nOracle:x\n")
	for p := range state.PlayerID(3) {
		fillLibrary(h.g, p, filler, 2)
	}
	src := h.g.AddObject(filler, 1)
	src.Zone = state.ZBattlefield
	src.Controller = 0
	if src.Owner != 1 || src.Controller != 0 {
		t.Fatalf("precondition: source owner=%d controller=%d, want 1 and 0", src.Owner, src.Controller)
	}
	return h, &Ctx{Source: src.ID, Controller: 0}, src
}

func handSizes(h *fakeHost) []int {
	out := make([]int, len(h.g.Players))
	for p := range out {
		out[p] = len(h.g.Zone(state.ZHand, state.PlayerID(p)))
	}
	return out
}

func TestDefinedPlayerReferentSelectors(t *testing.T) {
	h, c, _ := referentHost(t)

	grantor := h.g.AddObject(mkCard(t, "Name:Aura\nTypes:Enchantment\nOracle:x\n"), 2)
	grantor.Zone = state.ZBattlefield
	withGrantor := *c
	withGrantor.Grantor = grantor.ID

	trig := *c
	trig.TriggerContext.TriggerCardController = state.Target{Player: 1, IsPlayer: true}

	owned := h.g.AddObject(mkCard(t, "Name:Owned\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	rem := *c
	rem.Remembered = []state.Target{{Obj: owned.ID}}

	cases := []struct {
		name string
		line string
		ctx  *Ctx
		want []state.PlayerID
	}{
		{"CardOwner", "DB$ Draw | Defined$ CardOwner", c, []state.PlayerID{1}},
		{"OriginalHostController grantor", "DB$ Draw | Defined$ OriginalHostController", &withGrantor, []state.PlayerID{2}},
		{"OriginalHostController source", "DB$ Draw | Defined$ OriginalHostController", c, []state.PlayerID{0}},
		{"NonTriggeredCardController", "DB$ Draw | Defined$ NonTriggeredCardController", &trig, []state.PlayerID{0, 2}},
		{"NonRememberedOwner", "DB$ LoseLife | Defined$ NonRememberedOwner", &rem, []state.PlayerID{1, 2}},
		{"NonRememberedOwner nothing remembered", "DB$ LoseLife | Defined$ NonRememberedOwner", c, []state.PlayerID{0, 1, 2}},
		{"unknown referent stays unknown", "DB$ Draw | Defined$ NonNoSuchReferent", c, []state.PlayerID{0}},
	}
	for _, tc := range cases {
		if got := definedPlayers(h, tc.ctx, sa(t, tc.line)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if _, ok := knownDefinedTargets(h, c, "NonNoSuchReferent"); ok {
		t.Errorf("an unknown referent under Non must stay unknown")
	}
}

func svarOf(t *testing.T, card, name string) (*cards.Card, *cards.SA) {
	t.Helper()
	cd, ok := testutil.CorpusRegistry(t).Lookup(card)
	if !ok || len(cd.Faces) == 0 {
		t.Fatalf("corpus card %q missing", card)
	}
	s := cards.ResolveSVar(cd.Faces[0].SVars, name)
	if s == nil {
		t.Fatalf("%s has no SVar %s", card, name)
	}
	return cd, s
}

func TestDefinedPlayerReferentZenithChronicler(t *testing.T) {
	h, c, _ := referentHost(t)
	_, draw := svarOf(t, "Zenith Chronicler", "TrigDraw")
	c.TriggerContext.TriggerCardController = state.Target{Player: 1, IsPlayer: true}
	if got := handSizes(h); !slices.Equal(got, []int{0, 0, 0}) {
		t.Fatalf("precondition: hands %v", got)
	}
	Resolve(h, c, draw)
	if got := handSizes(h); !slices.Equal(got, []int{1, 0, 1}) {
		t.Fatalf("hands after Zenith draw = %v, want [1 0 1] (every player but the caster, seat 1)", got)
	}
}

func TestDefinedPlayerReferentGwenStacy(t *testing.T) {
	h, c, _ := referentHost(t)
	_, lose := svarOf(t, "The Death of Gwen Stacy", "DBLoseLife")
	discarded := h.g.AddObject(mkCard(t, "Name:Discarded\nTypes:Creature\nPT:1/1\nOracle:x\n"), 2)
	discarded.Zone = state.ZGraveyard
	c.Remembered = []state.Target{{Obj: discarded.ID}}
	Resolve(h, c, lose)
	loss := map[state.PlayerID]int{}
	for _, ev := range h.log {
		if ev.Kind == events.LifeChange {
			loss[ev.Player] += int(ev.Amount)
		}
	}
	if loss[0] != -3 || loss[1] != -3 || loss[2] != 0 {
		t.Fatalf("life changes = %v, want seats 0 and 1 at -3 and seat 2 untouched", loss)
	}
}

func TestDefinedPlayerReferentHoldForRansom(t *testing.T) {
	h, c, creature := referentHost(t)
	_, draw := svarOf(t, "Hold for Ransom", "DBDraw")
	// The enchanted creature (Source) is controlled by seat 1; the Aura
	// (Grantor) by seat 0.
	creature.Controller = 1
	aura := h.g.AddObject(mkCard(t, "Name:Aura\nTypes:Enchantment\nOracle:x\n"), 0)
	aura.Zone = state.ZBattlefield
	c.Source, c.Grantor, c.Controller = creature.ID, aura.ID, 1
	Resolve(h, c, draw)
	if got := handSizes(h); !slices.Equal(got, []int{1, 0, 0}) {
		t.Fatalf("hands = %v, want [1 0 0] (the Aura's controller draws, not the creature's)", got)
	}
}

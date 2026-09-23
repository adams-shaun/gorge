package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestRememberPeekedCapturesOnlyWhenRequested(t *testing.T) {
	for _, tc := range []struct {
		name, flag string
		want       int
	}{
		{"enabled", " | RememberPeeked$ True", 1},
		{"disabled", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, ids := twopilesBoard(t)
			ctx := &Ctx{Controller: 0, Source: ids[5]}
			Resolve(h, ctx, sa(t, "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | NoReveal$ True"+tc.flag))
			if h.g.Obj(ids[0]).Zone != state.ZLibrary {
				t.Fatal("peeked card left the library")
			}
			if got := len(ctx.Remembered); got != tc.want {
				t.Fatalf("Ctx.Remembered has %d entries, want %d", got, tc.want)
			}
			if got := len(h.g.Obj(ids[5]).Remembered); got != tc.want {
				t.Fatalf("source Remembered has %d entries, want %d", got, tc.want)
			}
			if tc.want == 1 && (ctx.Remembered[0].Obj != ids[0] || h.g.Obj(ids[5]).Remembered[0].Obj != ids[0]) {
				t.Fatalf("remembered wrong peeked card: ctx=%+v source=%+v", ctx.Remembered, h.g.Obj(ids[5]).Remembered)
			}
		})
	}
}

func TestTwoPilesLibraryZoneWithoutDefinedCardsStaysLoud(t *testing.T) {
	h, ids := twopilesBoard(t)
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:2]), SVars: twopilesSVars()}
	Resolve(h, ctx, sa(t, "DB$ TwoPiles | Defined$ You | Zone$ Battlefield | ValidCards$ Creature | Separator$ You | ChosenPile$ DBHand"))
	if h.g.Obj(ids[0]).Zone != state.ZLibrary || h.g.Obj(ids[1]).Zone != state.ZLibrary {
		t.Fatal("unsupported Zone shape moved cards")
	}
	if len(h.log) != 1 || h.log[0].Kind != events.Note || h.log[0].Text != "unimplemented TwoPiles shape: Zone$ Battlefield" {
		t.Fatalf("log = %+v, want exactly one Zone$ Note", h.log)
	}
}

func TestTwoPilesZoneLibraryAndFaceDownOneAreMetadata(t *testing.T) {
	h, ids := twopilesBoard(t)
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:2]), SVars: twopilesSVars()}
	Resolve(h, ctx, sa(t, "DB$ TwoPiles | Defined$ You | DefinedCards$ Remembered | Zone$ Library | FaceDown$ One | Separator$ You | ChosenPile$ DBHand | UnchosenPile$ DBGrave"))
	if h.g.Obj(ids[0]).Zone != state.ZHand || h.g.Obj(ids[1]).Zone != state.ZGraveyard {
		t.Fatalf("R-9 pile movement = %d, %d; want hand/graveyard", h.g.Obj(ids[0]).Zone, h.g.Obj(ids[1]).Zone)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("supported Zone/FaceDown metadata emitted Note: %+v", ev)
		}
	}
}

func TestTwoPilesFaceDownTrueStaysLoud(t *testing.T) {
	h, ids := twopilesBoard(t)
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:2]), SVars: twopilesSVars()}
	Resolve(h, ctx, sa(t, "DB$ TwoPiles | Defined$ You | DefinedCards$ Remembered | FaceDown$ True | Separator$ You | ChosenPile$ DBHand"))
	if h.g.Obj(ids[0]).Zone != state.ZLibrary || h.g.Obj(ids[1]).Zone != state.ZLibrary {
		t.Fatal("unsupported FaceDown shape moved cards")
	}
	if len(h.log) != 1 || h.log[0].Kind != events.Note || h.log[0].Text != "unimplemented TwoPiles shape: FaceDown$ True" {
		t.Fatalf("log = %+v, want exactly one FaceDown$ Note", h.log)
	}
}

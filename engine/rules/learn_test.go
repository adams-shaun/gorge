package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// learnCast resolves Cram Session for seat 0 with a Lesson (Environmental
// Sciences) and a non-Lesson (Grizzly Bears) in its sideboard, stopping at the
// pending learn choice.
func learnCast(t *testing.T) (*Engine, Config, *decision.Decision, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	_, cfg := searchEngine(t, reg, "Cram Session")
	cfg.Sideboards = [][]*cards.Card{{searchCorpusCard(t, reg, "Grizzly Bears"), searchCorpusCard(t, reg, "Environmental Sciences")}, nil}
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	var lesson state.ObjID
	for _, id := range e.G.Zone(state.ZSideboard, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Environmental Sciences" {
			lesson = id
		}
	}
	if lesson == 0 {
		t.Fatal("precondition: Environmental Sciences missing from the sideboard")
	}
	addMana(t, e, 0, "GG")
	id := searchMoveByName(t, e, "Cram Session", state.ZHand)
	d := castFixture(t, e, id, -1)
	// Cram Session's {B/G} pip asks which colour pays it; answer the first.
	for i := 0; i < 4 && d != nil && len(d.Options) > 0 && len(d.Options[0].Kind) > 4 && d.Options[0].Kind[:4] == "pay_"; i++ {
		submitChoices(t, e, d.Options[0].Index)
		d = e.Pending()
	}
	// Pass priority until Cram Session resolves into the learn choice.
	for i := 0; i < 4 && d != nil && d.Kind == decision.KPriority && len(e.G.Stack) > 0; i++ {
		submitChoices(t, e, 0)
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KChoose || d.Player != 0 {
		t.Fatalf("learn decision = %+v, want a seat-0 KChoose", d)
	}
	return e, cfg, d, lesson
}

func learnOption(t *testing.T, d *decision.Decision, kind string) decision.Option {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == kind {
			return o
		}
	}
	t.Fatalf("learn options %+v carry no %q branch", d.Options, kind)
	return decision.Option{}
}

func TestLearnOffersLessonRummageOrNothing(t *testing.T) {
	t.Parallel()

	t.Run("offers exactly the three branches", func(t *testing.T) {
		t.Parallel()
		_, _, d, lesson := learnCast(t)
		if len(d.Options) != 3 {
			t.Fatalf("learn options = %+v, want lesson, rummage, nothing (Grizzly Bears is no Lesson)", d.Options)
		}
		if o := learnOption(t, d, "lesson"); o.Obj != lesson {
			t.Fatalf("lesson option names %d, want Environmental Sciences %d", o.Obj, lesson)
		}
		learnOption(t, d, "rummage")
		learnOption(t, d, "nothing")
	})

	t.Run("lesson goes to hand revealed", func(t *testing.T) {
		t.Parallel()
		e, cfg, d, lesson := learnCast(t)
		start := len(e.L.Events)
		submitChoices(t, e, learnOption(t, d, "lesson").Index)
		passUntilStackEmpty(t, e, 20)
		if o := e.G.Obj(lesson); o == nil || o.Zone != state.ZHand || o.EnteredFrom != state.ZSideboard {
			t.Fatalf("Lesson = %+v, want in hand from the sideboard", o)
		}
		revealed := false
		for _, ev := range e.L.Events[start:] {
			if ev.Kind == events.Note && len(ev.IDs) == 1 && ev.IDs[0] == lesson && !ev.Secret {
				revealed = true
			}
		}
		if !revealed {
			t.Fatal("the Lesson was not publicly revealed")
		}
		replayCheck(t, e, cfg)
	})

	t.Run("rummage discards then draws", func(t *testing.T) {
		t.Parallel()
		e, cfg, d, _ := learnCast(t)
		handBefore := len(e.G.Zone(state.ZHand, 0))
		gyBefore := len(e.G.Zone(state.ZGraveyard, 0))
		libBefore := len(e.G.Zone(state.ZLibrary, 0))
		top := e.G.Zone(state.ZLibrary, 0)[0]
		submitChoices(t, e, learnOption(t, d, "rummage").Index)
		dd := e.Pending()
		if dd == nil || dd.Player != 0 || len(dd.Options) != handBefore {
			t.Fatalf("discard decision = %+v, want one option per card in hand (%d)", dd, handBefore)
		}
		pick := dd.Options[0]
		submitChoices(t, e, pick.Index)
		passUntilStackEmpty(t, e, 20)
		if o := e.G.Obj(pick.Obj); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("discarded card = %+v, want in the graveyard", o)
		}
		if o := e.G.Obj(top); o == nil || o.Zone != state.ZHand {
			t.Fatalf("drawn card = %+v, want the former top of library in hand", o)
		}
		if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore {
			t.Fatalf("hand size = %d, want %d (discard one, draw one)", got, handBefore)
		}
		// Cram Session itself also lands in the graveyard after resolving.
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != gyBefore+2 {
			t.Fatalf("graveyard size = %d, want %d", got, gyBefore+2)
		}
		if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore-1 {
			t.Fatalf("library size = %d, want %d", got, libBefore-1)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("nothing changes no zone", func(t *testing.T) {
		t.Parallel()
		e, cfg, d, lesson := learnCast(t)
		hand := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
		lib := len(e.G.Zone(state.ZLibrary, 0))
		submitChoices(t, e, learnOption(t, d, "nothing").Index)
		passUntilStackEmpty(t, e, 20)
		if o := e.G.Obj(lesson); o == nil || o.Zone != state.ZSideboard {
			t.Fatalf("Lesson = %+v, want still in the sideboard", o)
		}
		if got := e.G.Zone(state.ZHand, 0); len(got) != len(hand) {
			t.Fatalf("hand = %v, want %v", got, hand)
		}
		if got := len(e.G.Zone(state.ZLibrary, 0)); got != lib {
			t.Fatalf("library size = %d, want %d", got, lib)
		}
		replayCheck(t, e, cfg)
	})
}

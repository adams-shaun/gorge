package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Hostile Negotiations' REAL corpus card, end to end: two Digs put the top six
// cards face down into exile as Remembered (1-3) and Imprinted (4-6), the
// caster picks a pile (DefinedPiles$) to turn face up (SetState TurnFaceUp in
// exile), the opponent picks the pile that goes to hand, the rest to the
// graveyard, and the caster loses 3 life.

var hostileTopNames = []string{"Grizzly Bears", "Forest", "Mountain", "Island", "Swamp", "Plains"}

// hostileEngine casts Hostile Negotiations with a known six-card library top
// and drives to the first pending decision. Returns the engine, config, the
// six top ids in library order, the source id and the caster's starting life.
func hostileEngine(t *testing.T) (*Engine, Config, []state.ObjID, state.ObjID, int) {
	t.Helper()
	reg := searchTestRegistry(t)
	hn := searchCorpusCard(t, reg, "Hostile Negotiations")
	deck := []*cards.Card{hn}
	for _, n := range hostileTopNames {
		deck = append(deck, searchCorpusCard(t, reg, n))
	}
	for len(deck) < 40 {
		deck = append(deck, searchCorpusCard(t, reg, "Swamp"))
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = searchCorpusCard(t, reg, "Mountain")
	}
	cfg := seatZeroStart(Config{Seed: 9411, Names: []string{"hn", "opp"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	src := searchMoveByName(t, e, "Hostile Negotiations", state.ZHand)
	top := make([]state.ObjID, 0, 6)
	used := map[state.ObjID]bool{}
	for _, name := range hostileTopNames {
		id := state.ObjID(0)
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand} {
			for _, cand := range e.G.Zone(z, 0) {
				if o := e.G.Obj(cand); !used[cand] && o != nil && o.Face() != nil && o.Face().Name == name {
					id = cand
					break
				}
			}
			if id != 0 {
				break
			}
		}
		if id == 0 {
			t.Fatalf("top card %q absent from library/hand", name)
		}
		if e.G.Obj(id).Zone == state.ZHand {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
		used[id] = true
		top = append(top, id)
	}
	order := append([]state.ObjID(nil), top...)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if !used[id] {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order})
	addMana(t, e, 0, "BBBB")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == src {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Hostile Negotiations: %+v", d.Options)
	}
	life := int(e.G.Players[0].Life)
	submitChoices(t, e, idx)
	return e, cfg, top, src, life
}

func hostileFaceDown(t *testing.T, e *Engine, ids []state.ObjID, want bool) {
	t.Helper()
	for _, id := range ids {
		o := e.G.Obj(id)
		if o.Zone != state.ZExile || o.FaceDown != want {
			t.Fatalf("card %d: zone=%s faceDown=%v, want exile faceDown=%v", id, o.Zone, o.FaceDown, want)
		}
	}
}

func hostileNoUnimplemented(t *testing.T, e *Engine, start int) {
	t.Helper()
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			t.Fatalf("unimplemented Note after the cast: %q", ev.Text)
		}
		if ev.Kind == events.TurnFaceUp {
			t.Fatalf("battlefield TurnFaceUp emitted for an exiled card: %+v", ev)
		}
	}
}

func hostileFinish(t *testing.T, e *Engine, cfg Config, top []state.ObjID, src state.ObjID, life int, start int, handIDs, graveIDs []state.ObjID) {
	t.Helper()
	for _, id := range handIDs {
		if o := e.G.Obj(id); o.Zone != state.ZHand || o.Owner != 0 {
			t.Fatalf("card %d zone=%s, want seat 0's hand", id, o.Zone)
		}
	}
	for _, id := range graveIDs {
		if o := e.G.Obj(id); o.Zone != state.ZGraveyard || o.Owner != 0 {
			t.Fatalf("card %d zone=%s, want seat 0's graveyard", id, o.Zone)
		}
	}
	if got := int(e.G.Players[0].Life); got != life-3 {
		t.Fatalf("life = %d, want %d", got, life-3)
	}
	hostileNoUnimplemented(t, e, start)
	replayCheck(t, e, cfg)
}

func TestHostileNegotiationsPilesEndToEnd(t *testing.T) {
	t.Parallel()
	e, cfg, top, src, life := hostileEngine(t)
	start := len(e.L.Events)
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 0 || len(d.Options) != 2 {
		t.Fatalf("first decision = %+v, want the caster's twopiles_pick with 2 options", d)
	}
	hostileFaceDown(t, e, top, true)
	imp := e.G.Obj(src).Imprinted
	if len(imp) != 3 {
		t.Fatalf("Imprinted = %v, want cards 4-6", imp)
	}
	for i, id := range top[3:] {
		if imp[i] != id {
			t.Fatalf("Imprinted = %v, want %v", imp, top[3:])
		}
	}
	// Answer pile-b: the Imprinted pile (cards 4-6) turns face up.
	submitChoices(t, e, 1)
	hostileFaceDown(t, e, top[3:], false)
	hostileFaceDown(t, e, top[:3], true)
	d = e.Pending()
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 1 {
		t.Fatalf("second decision = %+v, want the opponent's twopiles_pick", d)
	}
	submitChoices(t, e, 0) // pile-a: cards 1-3 to hand, 4-6 to graveyard
	hostileFinish(t, e, cfg, top, src, life, start, top[:3], top[3:])
}

func TestHostileNegotiationsBotDefaultsTakePileA(t *testing.T) {
	t.Parallel()
	e, cfg, top, src, life := hostileEngine(t)
	start := len(e.L.Events)
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 0 {
		t.Fatalf("first decision = %+v, want the caster's twopiles_pick", d)
	}
	hostileFaceDown(t, e, top, true)
	submitChoices(t, e, 0)
	hostileFaceDown(t, e, top[:3], false)
	hostileFaceDown(t, e, top[3:], true)
	d = e.Pending()
	if d == nil || d.ResumeKind != "twopiles_pick" || d.Player != 1 || d.Kind != decision.KChoose {
		t.Fatalf("second decision = %+v, want the opponent's twopiles_pick", d)
	}
	submitChoices(t, e, 0)
	hostileFinish(t, e, cfg, top, src, life, start, top[:3], top[3:])
}

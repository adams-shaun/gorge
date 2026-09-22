package effects

// The sharesAllCardTypesWithOther predicate (Demonic Covenant's mill-sacrifice
// rider, SVar body `Remembered$Valid Card.sharesAllCardTypesWithOther
// Remembered`): the candidate shares EVERY one of its card types with some
// OTHER object the referent names. Classified in the SAME wordPredicate
// switch the matcher reads, so UnknownPredicates cannot disagree; fail-closed
// on an unbound referent. And ShowMilledCards$ True on the Mill primitive:
// one public ids-Note per acting player naming what was milled.

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// sharesAllFixture builds a 2-seat game with two Grizzly-Bear-shaped cards, a
// Forest-shaped card and a bear-land-shaped card (Creature Land) as objects.
func sharesAllFixture(t *testing.T) (*fakeHost, state.ObjID, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	bear1 := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	bear2 := h.g.AddObject(mkCard(t, "Name:Other Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	forest := h.g.AddObject(mkCard(t, "Name:Forest\nTypes:Land Forest\nOracle:x\n"), 0)
	bearLand := h.g.AddObject(mkCard(t, "Name:Bear Arbor\nTypes:Creature Land Forest Bear\nOracle:x\n"), 0)
	return h, bear1.ID, bear2.ID, forest.ID, bearLand.ID
}

// TestSharesAllCardTypesWithOtherClassified keeps the predicate silent to
// UnknownPredicates and its exotic spellings fail-closed-loud.
func TestSharesAllCardTypesWithOtherClassified(t *testing.T) {
	for _, ref := range []string{"Remembered", "RememberedCard", "RememberedLKI",
		"TriggeredCard", "TriggeredCardLKICopy", "Targeted", "Self", "Commander"} {
		spec := "Card.sharesAllCardTypesWithOther " + ref
		if got := UnknownPredicates(spec); len(got) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want []", spec, got)
		}
	}
	for _, spec := range []string{
		"Card.sharesAllCardTypesWith",
		"Card.sharesAllCardTypesWith Sacrificed",
		"Card.sharesAllCardTypesWithOther",
	} {
		if got := UnknownPredicates(spec); len(got) == 0 {
			t.Errorf("UnknownPredicates(%q) = [], want the token reported", spec)
		}
	}
}

// TestSharesAllCardTypesWithOtherMatches is the matcher half.
func TestSharesAllCardTypesWithOtherMatches(t *testing.T) {
	h, bear1, bear2, forest, bearLand := sharesAllFixture(t)
	g := h.g
	spec := "Card.sharesAllCardTypesWithOther Remembered"

	// Two bears: each shares all its card types (Creature) with the other.
	sc := SpecContext{You: 0, Remembered: []state.Target{{Obj: bear1}, {Obj: bear2}}}
	if !MatchesObjectCtx(g, spec, g.Obj(bear1), sc) {
		t.Errorf("bear 1 must share all its card types with bear 2")
	}
	if !MatchesObjectCtx(g, spec, g.Obj(bear2), sc) {
		t.Errorf("bear 2 must share all its card types with bear 1")
	}

	// Bear + Forest: neither shares all its types with the other (the bear's
	// Creature is missing from the Forest, the Forest's Land from the bear).
	sc = SpecContext{You: 0, Remembered: []state.Target{{Obj: bear1}, {Obj: forest}}}
	if MatchesObjectCtx(g, spec, g.Obj(bear1), sc) {
		t.Errorf("the bear must not share all its card types with the Forest")
	}
	if MatchesObjectCtx(g, spec, g.Obj(forest), sc) {
		t.Errorf("the Forest must not share all its card types with the bear")
	}

	// Subset direction (Forge's allMatch over the CANDIDATE's types): a bare
	// Forest shares all its types (Land) with the bear-land; the bear-land
	// does not share all of its types (Creature AND Land) with the bare
	// Forest.
	sc = SpecContext{You: 0, Remembered: []state.Target{{Obj: forest}, {Obj: bearLand}}}
	if !MatchesObjectCtx(g, spec, g.Obj(forest), sc) {
		t.Errorf("the Forest must share all its card types (Land) with the bear-land")
	}
	if MatchesObjectCtx(g, spec, g.Obj(bearLand), sc) {
		t.Errorf("the bear-land must not share ALL its card types with the bare Forest")
	}

	// No OTHER: a remembered set holding only the candidate matches nothing.
	sc = SpecContext{You: 0, Remembered: []state.Target{{Obj: bear1}}}
	if MatchesObjectCtx(g, spec, g.Obj(bear1), sc) {
		t.Errorf("a candidate with no OTHER remembered card must match nothing")
	}

	// Unbound referent: fail closed, never widened.
	sc = SpecContext{You: 0}
	if MatchesObjectCtx(g, spec, g.Obj(bear1), sc) {
		t.Errorf("an unbound referent must match nothing (fail closed)")
	}
}

// TestRememberedValidSharesAllCount evaluates the carrier's real SVar body
// through the count head: two bears count 2 (the gate's GE2 holds), bear plus
// Forest counts 0.
func TestRememberedValidSharesAllCount(t *testing.T) {
	h, bear1, bear2, forest, _ := sharesAllFixture(t)
	body := "Remembered$Valid Card.sharesAllCardTypesWithOther Remembered"

	c := &Ctx{Controller: 0, Remembered: []state.Target{{Obj: bear1}, {Obj: bear2}}}
	if n, ok := EvalCountOK(h, c, body); !ok || n != 2 {
		t.Fatalf("EvalCountOK(two bears) = %d,%v, want 2,true", n, ok)
	}
	c = &Ctx{Controller: 0, Remembered: []state.Target{{Obj: bear1}, {Obj: forest}}}
	if n, ok := EvalCountOK(h, c, body); !ok || n != 0 {
		t.Fatalf("EvalCountOK(bear, forest) = %d,%v, want 0,true", n, ok)
	}
}

// TestSacrificeShowSacrificedCardsRevealsPublicly is the twin on the
// Sacrifice primitive (Demonic Covenant's DB$ Sacrifice line carries
// ShowSacrificedCards$ True): one public Note naming the sacrificed object.
// Without the param no Note is emitted.
func TestSacrificeShowSacrificedCardsRevealsPublicly(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Oblation Fodder\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	src.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append([]state.ObjID{src.ID}, h.g.Zone(state.ZBattlefield, 0)...))

	c := &Ctx{Source: src.ID, Controller: 0}
	Resolve(h, c, sa(t, "DB$ Sacrifice | SacValid$ Self | ShowSacrificedCards$ True"))

	var notes []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			notes = append(notes, ev)
		}
	}
	if len(notes) != 1 {
		t.Fatalf("Note events = %d, want exactly the one reveal (%+v)", len(notes), h.log)
	}
	if notes[0].Secret || len(notes[0].IDs) != 1 || notes[0].IDs[0] != src.ID {
		t.Fatalf("reveal Note = %+v, want a public Note naming %d", notes[0], src.ID)
	}
	if o := h.g.Obj(src.ID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("sacrificed object = %+v, want in the graveyard", o)
	}

	// Negative: the same Sacrifice without ShowSacrificedCards$ emits no Note.
	h2 := newHost(t, 2)
	src2 := h2.g.AddObject(mkCard(t, "Name:Oblation Fodder\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	src2.Zone = state.ZBattlefield
	h2.g.SetZone(state.ZBattlefield, 0, append([]state.ObjID{src2.ID}, h2.g.Zone(state.ZBattlefield, 0)...))
	Resolve(h2, &Ctx{Source: src2.ID, Controller: 0}, sa(t, "DB$ Sacrifice | SacValid$ Self"))
	for _, ev := range h2.log {
		if ev.Kind == events.Note {
			t.Fatalf("no ShowSacrificedCards$ but a Note was emitted: %+v", ev)
		}
	}
}

// TestMillShowMilledCardsRevealsPublicly drives the real Mill primitive with
// RememberMilled$ + ShowMilledCards$: the two milled cards move to the
// graveyard, and ONE public Note (not Secret) names them. Without the param
// no Note is emitted.
func TestMillShowMilledCardsRevealsPublicly(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Millstone\nTypes:Artifact\nOracle:x\n"), 0)
	bear := mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	ids := fillLibrary(h.g, 0, bear, 2)

	c := &Ctx{Source: src.ID, Controller: 0}
	Resolve(h, c, sa(t, "DB$ Mill | NumCards$ 2 | RememberMilled$ True | ShowMilledCards$ True"))

	if len(c.Remembered) != 2 {
		t.Fatalf("remembered = %+v, want the two milled cards", c.Remembered)
	}
	var notes []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			notes = append(notes, ev)
		}
	}
	if len(notes) != 1 {
		t.Fatalf("Note events = %d (%+v), want exactly the one reveal", len(notes), h.log)
	}
	note := notes[0]
	if note.Secret || note.Player != 0 || len(note.IDs) != 2 || note.IDs[0] != ids[0] || note.IDs[1] != ids[1] {
		t.Fatalf("reveal Note = %+v, want a public Note by seat 0 naming %v", note, ids)
	}
	for _, id := range ids {
		if o := h.g.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("milled card %d = %+v, want in the graveyard", id, o)
		}
	}

	// Negative: the same Mill without ShowMilledCards$ emits no Note.
	h2 := newHost(t, 2)
	fillLibrary(h2.g, 0, bear, 2)
	c2 := &Ctx{Source: src.ID, Controller: 0}
	Resolve(h2, c2, sa(t, "DB$ Mill | NumCards$ 2 | RememberMilled$ True"))
	for _, ev := range h2.log {
		if ev.Kind == events.Note {
			t.Fatalf("no ShowMilledCards$ but a Note was emitted: %+v", ev)
		}
	}
}

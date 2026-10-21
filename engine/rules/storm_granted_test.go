package rules

// Prismari, the Inspiration's "Instant and sorcery spells you cast have
// storm" is a layer-6 AddKeyword$ Storm grant scoped to AffectedZone$ Stack.
// CR 702.40a: "Storm is a triggered ability that functions on the stack.
// 'Storm' means 'When you cast this spell, copy it for each other spell that
// was cast before it this turn.'" The printed K:Storm expansion only covered
// printed lines, so a granted Storm put no trigger on the stack (level-B
// compliance row SOS Prismari static#0.0).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const stormProbeSpell = "Name:Mend\nManaCost:R\nTypes:Instant\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:You gain 1 life.\n"

func castMendTwice(t *testing.T, prismari bool) (*Engine, int) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Prismari, the Inspiration"), card(t, stormProbeSpell), card(t, stormProbeSpell))
	hand := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
	if prismari {
		e.emit(events.Event{Kind: events.MoveZone, Obj: hand[0], From: state.ZHand, To: state.ZBattlefield})
	}
	spells := hand[1:]
	addMana(t, e, 0, "RR")
	triggers := 0
	for _, id := range spells {
		opt := offeredCast(e, id)
		if opt < 0 {
			t.Fatalf("Mend not offered")
		}
		submitChoices(t, e, opt)
		for _, sid := range e.G.Stack {
			if so := e.G.Obj(sid); so != nil && so.Ability != nil && so.Source == id {
				triggers++
			}
		}
		passUntilStackEmpty(t, e, 20)
	}
	return e, triggers
}

func TestGrantedStormCopiesForEachEarlierSpell(t *testing.T) {
	t.Parallel()
	e, triggers := castMendTwice(t, true)
	if triggers != 2 {
		t.Fatalf("CR 702.40a: %d storm triggers on the stack, want one per granted cast", triggers)
	}
	// First Mend: storm count 0. Second Mend: one copy. 1 + 1 + 1 life.
	if life := e.G.Players[0].Life; life != 23 {
		t.Fatalf("life=%d, want 23 (two casts plus one storm copy)", life)
	}
}

func TestGrantedStormAbsentWithoutTheGrant(t *testing.T) {
	t.Parallel()
	e, triggers := castMendTwice(t, false)
	if triggers != 0 {
		t.Fatalf("%d storm triggers without the grant", triggers)
	}
	if life := e.G.Players[0].Life; life != 22 {
		t.Fatalf("life=%d, want 22", life)
	}
}

// TestGrantedStormCountsSpellsCastBeforeIt: CR 702.40a's count is fixed when
// the trigger fires, granted storm included (the fire-time capture keys on the
// Keyword$ Storm stamp the synthesis carries). First Mend, then a second Mend
// cast IN RESPONSE to the first's storm trigger: the first trigger's count is
// one spell (itself), so it copies nothing; only the second trigger copies.
// 1 + 1 + 1 life -- the response must not add a copy to the first trigger.
func TestGrantedStormCountsSpellsCastBeforeIt(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Prismari, the Inspiration"), card(t, stormProbeSpell), card(t, stormProbeSpell))
	hand := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
	e.emit(events.Event{Kind: events.MoveZone, Obj: hand[0], From: state.ZHand, To: state.ZBattlefield})
	spells := hand[1:]
	addMana(t, e, 0, "RR")
	triggerStackObjects := func() int {
		n := 0
		for _, sid := range e.G.Stack {
			if so := e.G.Obj(sid); so != nil && so.Ability != nil && (so.Source == spells[0] || so.Source == spells[1]) {
				n++
			}
		}
		return n
	}
	// First Mend; its storm trigger stays on the stack.
	if opt := offeredCast(e, spells[0]); opt < 0 {
		t.Fatalf("first Mend not offered")
	} else {
		submitChoices(t, e, opt)
	}
	if n := triggerStackObjects(); n != 1 {
		t.Fatalf("CR 702.40a seq %d: want the granted storm trigger on the stack with the spell, got %d ability objects: %v", len(e.L.Events), n, e.G.Stack)
	}
	// Second Mend cast in response to the first's trigger.
	if opt := offeredCast(e, spells[1]); opt < 0 {
		t.Fatalf("second Mend not offered")
	} else {
		submitChoices(t, e, opt)
	}
	if n := triggerStackObjects(); n != 2 {
		t.Fatalf("CR 702.40a seq %d: want both storm triggers on the stack with the second spell, got %d triggers: %v", len(e.L.Events), n, e.G.Stack)
	}
	passUntilStackEmpty(t, e, 20)
	if life := e.G.Players[0].Life; life != 23 {
		t.Fatalf("life=%d, want 23 (two casts plus the second trigger's one copy; a response must not add a copy to the first trigger)", life)
	}
}

// TestGrantedStormReplaysFromTheLog: the granted trigger's body crosses the
// log as the __kwStorm: payload, so a log-only replay mints the identical
// ability and copy.
func TestGrantedStormReplaysFromTheLog(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	prismari := searchCorpusCard(t, reg, "Prismari, the Inspiration")
	mend := card(t, stormProbeSpell)
	e, cfg := demonstrateEngine(t, reg, []*cards.Card{prismari, mend, mend}, nil)
	demonstrateMove(t, e, 0, "Prismari, the Inspiration", state.ZBattlefield)
	var spells []state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mend" {
			spells = append(spells, id)
		}
	}
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mend" {
			spells = append(spells, id)
		}
	}
	if len(spells) != 2 {
		t.Fatalf("found %d Mends, want 2", len(spells))
	}
	for _, id := range spells {
		if o := e.G.Obj(id); o.Zone != state.ZHand {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZHand})
		}
	}
	e.pending = nil
	e.priorityRound()
	addMana(t, e, 0, "RR")
	for _, id := range spells {
		opt := offeredCast(e, id)
		if opt < 0 {
			t.Fatalf("Mend not offered")
		}
		submitChoices(t, e, opt)
		drainStack(t, e, 20)
	}
	if life := e.G.Players[0].Life; life != 23 {
		t.Fatalf("life=%d, want 23 (two casts plus one storm copy)", life)
	}
	replayCheck(t, e, cfg)
}

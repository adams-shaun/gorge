package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// entreatShape is Entreat the Angels' resolved shape: a sorcery whose body
// creates TokenAmount$ X 4/4 flying Angels, where X is the value paid for
// the mana cost's {X}{X}{W}{W}{W} through SVar:X:Count$xPaid. The token
// script is registered by newFixtureDeckWithTokens (the same registration
// the miracle test's Entreat shares).
const entreatShape = "Name:Entreat\nManaCost:X X W W W\nTypes:Sorcery\n" +
	"A:SP$ Token | TokenAmount$ X | TokenScript$ w_4_4_angel_flying | TokenOwner$ You\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// countAngelTokens counts seat p's battlefield Angels named "Angel Token".
func countAngelTokens(t *testing.T, e *Engine, p state.PlayerID) int {
	t.Helper()
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if f := e.G.Obj(id).Face(); f != nil && f.Name == "Angel Token" {
			n++
		}
	}
	return n
}

// TestEntreatTheAngelsCreatesItsChosenNumberOfAngels is CR 107.3i end to
// end: the value chosen for a spell's {X} is what every numeric parameter
// that reads the paid X sees as the spell resolves. resolveTop's spell
// branch binds the stack object's CastInfo-recorded X (events.CastInfo ->
// o.X) into effects.Ctx.X, so SVar:X:Count$xPaid answers the chosen value
// and TokenAmount$ X creates that many tokens -- not zero, which is what a
// Ctx without the binding made Entreat resolve to.
func TestEntreatTheAngelsCreatesItsChosenNumberOfAngels(t *testing.T) {
	e, cfg, id := newFixtureDeckWithTokens(t, 111, entreatShape)
	addMana(t, e, 0, "CCCCWWW") // X = 2: {X}{X} folds to {2}{2} generic + {W}{W}{W}
	opts := castOptions(t, e)
	idx := -1
	for _, o := range opts {
		if o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Entreat: %+v", opts)
	}
	submitChoices(t, e, idx)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" {
		t.Fatalf("X decision %+v", d)
	}
	submitChoices(t, e, 2)
	passUntilStackEmpty(t, e, 20)
	if got := countAngelTokens(t, e, 0); got != 2 {
		t.Fatalf("Angels on the battlefield = %d, want 2", got)
	}
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("Entreat zone %s after resolving", e.G.Obj(id).Zone)
	}
	replayCheck(t, e, cfg)
}

// TestPaidXSurvivesAMidResolutionSuspension pins the resume arm of the same
// binding: a spell whose resolution suspends on a mid-resolution ask keeps
// the paid X for the rest of the walk. The fixture discards one card from
// the opponent's hand (which suspends on the discard ask) and then, via its
// SubAbility, creates TokenAmount$ X Angels -- so the token count proves the
// resumed Ctx still carries X = 2.
func TestPaidXSurvivesAMidResolutionSuspension(t *testing.T) {
	src := "Name:Torment\nManaCost:X B\nTypes:Sorcery\n" +
		"A:SP$ Discard | Mode$ TgtChoose | NumCards$ 1 | ValidTgts$ Opponent | SubAbility$ Angels | SpellDescription$ x\n" +
		"SVar:Angels:DB$ Token | TokenAmount$ X | TokenScript$ w_4_4_angel_flying | TokenOwner$ You\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	e, cfg, id := newFixtureDeckWithTokens(t, 112, src)
	// The fixture deals no opening hand: move two Mountains from seat 1's
	// library into their hand via logged MoveZones, so the discard ask has
	// eligible cards to offer (the ask fires only when eligible > NumCards).
	for _, cid := range e.G.Zone(state.ZLibrary, 1)[:2] {
		e.emit(events.Event{Kind: events.MoveZone, Obj: cid, From: state.ZLibrary, To: state.ZHand})
	}
	opponentHand := len(e.G.Zone(state.ZHand, 1))
	if opponentHand < 2 {
		t.Fatalf("opponent hand %d, need 2+ eligible for the discard ask", opponentHand)
	}
	addMana(t, e, 0, "CCB") // X = 2: {2}{B}
	opts := castOptions(t, e)
	idx := -1
	for _, o := range opts {
		if o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Torment: %+v", opts)
	}
	submitChoices(t, e, idx)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" {
		t.Fatalf("X decision %+v", d)
	}
	submitChoices(t, e, 2)
	// Target ask: pick the opponent's option (the option carrying Player 1).
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("target decision %+v", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Player == 1 {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no opponent option in %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	// Payment returns to priority; both seats pass so the spell resolves.
	submitChoices(t, e, 0)
	d = e.Pending()
	if d == nil || d.Player != 1 || d.Kind != decision.KPriority {
		t.Fatalf("seat 1 priority %+v", d)
	}
	submitChoices(t, e, 0)
	// The discard ask: the discarding player (the opponent) picks 1 of their
	// own cards, which suspends resolution until answered.
	d = e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "discard" {
		t.Fatalf("discard decision %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 20)
	if got := len(e.G.Zone(state.ZHand, 1)); got != opponentHand-1 {
		t.Fatalf("opponent hand %d, want %d after the discard", got, opponentHand-1)
	}
	if got := countAngelTokens(t, e, 0); got != 2 {
		t.Fatalf("Angels on the battlefield = %d, want 2 (paid X lost across the suspension?)", got)
	}
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("Torment zone %s after resolving", e.G.Obj(id).Zone)
	}
	replayCheck(t, e, cfg)
}

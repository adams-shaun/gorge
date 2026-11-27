package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.132 Assist, pinned end to end on the REAL corpus carriers. Bring
// Down ({3}{W}, "Destroy target creature with power 4 or greater", K:Assist)
// is the printed carrier and Gang Up ({X}{B}, "Destroy target creature with
// power X or less", K:Assist) the X carrier, so the assist amount is read both
// from a printed generic part and from the announced X. Four seats are dealt
// because commander — the format that prints Assist — is four players.
//
// The decks are built from compiled corpus cards only, so no Forge script text
// is committed. The helpers come from search_library_test.go
// (searchTestRegistry, searchCorpusCard, searchMoveByName), cast_test.go
// (addMana, submitChoices, toMain1), token_replacement_test.go
// (moveSeededCard), multikicker_test.go (castOptMode) and
// replacement_updated_test.go (passUntilStackEmpty) — all the same package.

// assistEngine deals four seats a 40-card Mountain deck, seat 0 led by the
// named Assist carrier and seat 1 carrying the two creatures the tests seed
// (Bonebreaker Giant, power 4, is Bring Down's legal target; Grizzly Bears,
// power 2, is Gang Up's at X = 2). Cards reach the battlefield through
// moveSeededCard (the seeded harness, a real logged MoveZone), so every event
// in the log is engine-produced and replayCheck stays honest.
func assistEngine(t *testing.T, hero string) (*Engine, Config, *cards.Registry) {
	t.Helper()
	reg := searchTestRegistry(t)
	mountain := searchCorpusCard(t, reg, "Mountain")
	lead := searchCorpusCard(t, reg, hero)
	giant := searchCorpusCard(t, reg, "Bonebreaker Giant")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := func(extra ...*cards.Card) []*cards.Card {
		out := append([]*cards.Card(nil), extra...)
		for len(out) < 40 {
			out = append(out, mountain)
		}
		return out
	}
	cfg := seatZeroStart(Config{Seed: 7142026, Names: []string{"caster", "left", "payer", "right"},
		Decks: [][]*cards.Card{deck(lead), deck(giant, bear), deck(), deck()}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg, reg
}

// assistSeedBattlefield puts one copy of the named corpus card onto seat p's
// battlefield through the seeded harness and returns the object id.
func assistSeedBattlefield(t *testing.T, e *Engine, reg *cards.Registry, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	id := moveSeededCard(t, e, p, searchCorpusCard(t, reg, name), state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != p {
		t.Fatalf("precondition: %s is not on seat %d's battlefield: %+v", name, p, o)
	}
	return id
}

// assistPool is the mana a seat's pool holds, summed over every slot.
func assistPool(e *Engine, p state.PlayerID) int32 {
	var n int32
	for _, v := range e.G.Players[p].Pool {
		n += v
	}
	return n
}

// assistSpent is the mana seat p has actually spent: minus the sum of the
// negative ManaAdd events its payment emitted (the shape PayMana spends with).
func assistSpent(e *Engine, p state.PlayerID) int32 {
	var n int32
	for _, ev := range e.L.Events {
		if ev.Kind == events.ManaAdd && ev.Player == p && ev.Amount < 0 {
			n -= ev.Amount
		}
	}
	return n
}

// isAsk reports whether the pending decision is the Assist amount election.
func isAssistAsk(d *decision.Decision) bool {
	return d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "assist"
}

// assistAsk returns the pending Assist election asked of seat p, failing when
// the pending decision is not that election or names another seat.
func assistAsk(t *testing.T, e *Engine, p state.PlayerID) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if !isAssistAsk(d) {
		t.Fatalf("expected an assist KChoose, got %+v", d)
	}
	if d.Player != p {
		t.Fatalf("assist ask asked seat %d, want seat %d: %+v", d.Player, p, d)
	}
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("assist ask Min/Max = %d/%d, want 1/1: %+v", d.Min, d.Max, d)
	}
	return d
}

// assistAmounts returns the amounts the pending election offers.
func assistAmounts(t *testing.T, e *Engine, p state.PlayerID) []int {
	t.Helper()
	d := assistAsk(t, e, p)
	var out []int
	for _, o := range d.Options {
		out = append(out, o.Amount)
	}
	return out
}

// assistAnswer answers the pending Assist election with the option paying n.
func assistAnswer(t *testing.T, e *Engine, n int) {
	t.Helper()
	d := e.Pending()
	if !isAssistAsk(d) {
		t.Fatalf("no assist election to answer (got %+v)", d)
	}
	for _, o := range d.Options {
		if o.Amount == n {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("assist option paying %d not offered: %+v", n, d.Options)
}

// answerAssistTarget answers a pending target decision naming id. A forced
// single-target election is not posed at all, so a missing target decision is
// not an error here: the outcome assertions pin the target either way.
func answerAssistTarget(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		return
	}
	for _, o := range d.Options {
		if o.Obj == id {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("target %d not offered: %+v", id, d.Options)
}

// answerAssistX answers the pending {X} announcement with n.
func answerAssistX(t *testing.T, e *Engine, n int) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("expected an x election, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Amount == n {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("x value %d not offered: %+v", n, d.Options)
}

// TestAssistPaysGenericCost is the CR 702.132a contract on Bring Down
// ({3}{W}): the seat that can pay is offered 0..3, pays all {3} of the generic
// part out of its OWN pool, and the caster then pays only the {W} that is left
// — 3 mana out of seat 2's pool, 1 out of the caster's, and the target
// destroyed. With nothing left of the generic part, the next seat is not asked.
func TestAssistPaysGenericCost(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Bring Down")
	giant := assistSeedBattlefield(t, e, reg, 1, "Bonebreaker Giant")
	hero := searchMoveByName(t, e, "Bring Down", state.ZHand)
	addMana(t, e, 0, "WWWW") // {3}{W}
	addMana(t, e, 2, "CCC")  // seat 2 can fund the whole assist
	if got := assistPool(e, 2); got != 3 {
		t.Fatalf("precondition: seat 2's pool holds %d, want 3", got)
	}
	if got := assistPool(e, 0); got != 4 {
		t.Fatalf("precondition: the caster's pool holds %d, want 4", got)
	}

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistTarget(t, e, giant)

	if got := assistAmounts(t, e, 2); len(got) != 4 || got[0] != 0 || got[3] != 3 {
		t.Fatalf("assist offers %v, want [0 1 2 3]", got)
	}
	assistAnswer(t, e, 3)
	if isAssistAsk(e.Pending()) {
		t.Fatalf("a seat with nothing left of the generic cost was still asked: %+v", e.Pending())
	}
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(giant); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Bring Down did not destroy its target: %+v", o)
	}
	if got := assistSpent(e, 2); got != 3 {
		t.Fatalf("the assisting seat spent %d, want 3", got)
	}
	if got := assistPool(e, 2); got != 0 {
		t.Fatalf("the assisting seat's pool holds %d, want 0", got)
	}
	if got := assistSpent(e, 0); got != 1 {
		t.Fatalf("the caster spent %d, want 1 (the {W} left after the assist)", got)
	}
	if got := assistPool(e, 0); got != 3 {
		t.Fatalf("the caster's pool holds %d, want 3", got)
	}
	replayCheck(t, e, cfg)
}

// TestAssistDeclinedPaysFullCost is the decline direction: every seat answers 0
// (and a seat with no mana is not asked at all), so the caster pays the whole
// {3}{W} and no foreign pool moves.
func TestAssistDeclinedPaysFullCost(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Bring Down")
	giant := assistSeedBattlefield(t, e, reg, 1, "Bonebreaker Giant")
	hero := searchMoveByName(t, e, "Bring Down", state.ZHand)
	addMana(t, e, 0, "WWWW")
	addMana(t, e, 2, "CCC")

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistTarget(t, e, giant)
	assistAnswer(t, e, 0)
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(giant); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Bring Down did not destroy its target: %+v", o)
	}
	if got := assistSpent(e, 0); got != 4 {
		t.Fatalf("the caster spent %d, want 4 (the full {3}{W})", got)
	}
	if got := assistSpent(e, 2); got != 0 {
		t.Fatalf("a declining seat spent %d, want 0", got)
	}
	if got := assistPool(e, 2); got != 3 {
		t.Fatalf("a declining seat's pool holds %d, want 3", got)
	}
	replayCheck(t, e, cfg)
}

// TestAssistCapsAtPayerPool pins the offer ceiling: a seat holding {2} is
// offered 0..2 and never more, so its contribution can only ever be mana it
// can actually pay; the caster pays the remaining {1}{W}.
func TestAssistCapsAtPayerPool(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Bring Down")
	giant := assistSeedBattlefield(t, e, reg, 1, "Bonebreaker Giant")
	hero := searchMoveByName(t, e, "Bring Down", state.ZHand)
	addMana(t, e, 0, "WWWW")
	addMana(t, e, 2, "CC")

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistTarget(t, e, giant)
	if got := assistAmounts(t, e, 2); len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Fatalf("assist offers %v, want [0 1 2] (capped at the payer's pool)", got)
	}
	assistAnswer(t, e, 2)
	passUntilStackEmpty(t, e, 40)

	if got := assistSpent(e, 2); got != 2 {
		t.Fatalf("the assisting seat spent %d, want 2", got)
	}
	if got := assistSpent(e, 0); got != 2 {
		t.Fatalf("the caster spent %d, want 2 (the {1}{W} left)", got)
	}
	replayCheck(t, e, cfg)
}

// TestAssistAsksEachSeatInAPNAPRound is the four-seat contract: with one mana
// in each other seat's pool every seat is asked, in APNAP order from the caster
// (1, then 2, then 3), and the three contributions together fund the whole
// generic part — the caster pays only the {W}.
func TestAssistAsksEachSeatInAPNAPRound(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Bring Down")
	giant := assistSeedBattlefield(t, e, reg, 1, "Bonebreaker Giant")
	hero := searchMoveByName(t, e, "Bring Down", state.ZHand)
	addMana(t, e, 0, "WWWW")
	addMana(t, e, 1, "C")
	addMana(t, e, 2, "C")
	addMana(t, e, 3, "C")

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistTarget(t, e, giant)
	for _, p := range []state.PlayerID{1, 2, 3} {
		assistAsk(t, e, p)
		assistAnswer(t, e, 1)
	}
	passUntilStackEmpty(t, e, 40)

	for _, p := range []state.PlayerID{1, 2, 3} {
		if got := assistSpent(e, p); got != 1 {
			t.Fatalf("seat %d spent %d, want 1", p, got)
		}
	}
	if got := assistSpent(e, 0); got != 1 {
		t.Fatalf("the caster spent %d, want 1 (the {W} left after {3} of assists)", got)
	}
	if o := e.G.Obj(giant); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Bring Down did not destroy its target: %+v", o)
	}
	replayCheck(t, e, cfg)
}

// TestAssistXCardUsesAnnouncedX is the X carrier (Gang Up, {X}{B}): the assist
// amount is the announced X, not a printed number. At X = 2 the payer is
// offered 0..2, pays {2}, and the caster pays the {B} alone.
func TestAssistXCardUsesAnnouncedX(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Gang Up")
	bear := assistSeedBattlefield(t, e, reg, 1, "Grizzly Bears")
	hero := searchMoveByName(t, e, "Gang Up", state.ZHand)
	addMana(t, e, 0, "BBC") // {X}{B} at X = 2
	addMana(t, e, 2, "CC")

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistX(t, e, 2)
	answerAssistTarget(t, e, bear)

	if got := assistAmounts(t, e, 2); len(got) != 3 || got[2] != 2 {
		t.Fatalf("assist offers %v, want [0 1 2] (the announced X)", got)
	}
	assistAnswer(t, e, 2)
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Gang Up did not destroy its target: %+v", o)
	}
	if got := assistSpent(e, 2); got != 2 {
		t.Fatalf("the assisting seat spent %d, want 2", got)
	}
	if got := assistSpent(e, 0); got != 1 {
		t.Fatalf("the caster spent %d, want 1 (the {B} left)", got)
	}
	replayCheck(t, e, cfg)
}

package rules

// stat:SurveilNum — "You may look at an additional two cards each time you
// surveil" (Enhanced Surveillance). Before this ticket the mode was unread:
// effects.Supported() did not report it, nothing collected the static, and
// effects' effSurveil always used the surveil instruction's own count. The
// reader is rules.Engine.SurveilLookExtra (the canonical activeStatics walk,
// exposed to effects through the Host interface), the election is a real
// yes/no ask, and the no-host path declines deterministically (R-9).

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// surveilOneSrc is a freely-authored Surveil 1: the base count the static
// raises. The real corpus card under test is Enhanced Surveillance (looked up
// from .cards, never inlined).
const surveilOneSrc = "Name:Surveil One\nManaCost:U\nTypes:Sorcery\n" +
	"A:SP$ Surveil | Defined$ You | Amount$ 1\nOracle:x\n"

// surveilNumFixture seats the REAL corpus Enhanced Surveillance on seat 0's
// battlefield and an authored Surveil 1 in its hand, and returns the engine,
// the config (for replayCheck), the Surveil 1 card id and the Enhanced
// Surveillance permanent id.
func surveilNumFixture(t *testing.T, seed uint64) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	es, ok := reg.Lookup("Enhanced Surveillance")
	if !ok {
		t.Fatal("corpus fixture: Enhanced Surveillance missing")
	}
	if d := es.Link(); len(d) != 0 {
		t.Fatalf("link Enhanced Surveillance: %v", d)
	}
	src := card(t, surveilOneSrc)
	m, ok := reg.Lookup("Mountain")
	if !ok {
		t.Fatal("corpus fixture: Mountain missing")
	}
	fill := func(n int) []*cards.Card {
		out := make([]*cards.Card, n)
		for i := range out {
			out[i] = m
		}
		return out
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{es, src}, fill(38)...),
			fill(40),
		}})
	e := New(cfg)
	e.Advance()
	esID := moveByName(t, e, 0, "Enhanced Surveillance", state.ZBattlefield)
	id := moveSurveilToHand(t, e, 0, "Surveil One")
	addMana(t, e, 0, "U")
	return e, cfg, id, esID
}

// moveSurveilToHand puts the named card in p's hand without a self-move:
// moveByName would emit a hand->hand MoveZone when the card was already
// dealt into the opening hand.
func moveSurveilToHand(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	var id state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, cid := range e.G.Zone(z, p) {
			o := e.G.Obj(cid)
			if o == nil || o.Face() == nil || o.Face().Name != name {
				continue
			}
			id = cid
			if z != state.ZHand {
				e.emit(events.Event{Kind: events.MoveZone, Obj: cid, From: z, To: state.ZHand})
			}
		}
	}
	if id == 0 {
		t.Fatalf("card %q not dealt to seat %d", name, p)
	}
	return id
}

// TestEnhancedSurveillanceRaisesSurveilCountByTwo is the core leaf: with the
// real Enhanced Surveillance on the battlefield, a Surveil 1 poses its
// may-look election first; accepting it makes the KArrange look at THREE
// cards (1 + 2), where without the readable static it looks at one.
func TestEnhancedSurveillanceRaisesSurveilCountByTwo(t *testing.T) {
	e, cfg, id, esID := surveilNumFixture(t, 601)
	// Precondition: the carrier is really on seat 0's battlefield under its
	// controller -- the static has nowhere else to apply from.
	if o := e.G.Obj(esID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Enhanced Surveillance %v not on seat 0's battlefield: %+v", esID, e.G.Obj(esID))
	}
	// Precondition: at least three cards in the library, or the +2 would be
	// clipped and the Max==3 assertion below would pass for the wrong reason.
	if n := len(e.G.Zone(state.ZLibrary, 0)); n < 3 {
		t.Fatalf("precondition: seat 0 library has %d cards, need >= 3 to distinguish a 1-card look from a 3-card look", n)
	}

	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the stat:SurveilNum may-look election first, got %+v", d)
	}
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 2 ||
		d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("election = %+v, want Min/Max 1/1 with yes/no options", d)
	}
	submitChoices(t, e, 0) // yes: look at the additional cards

	d = e.Pending()
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the KArrange after accepting the extra look, got %+v", d)
	}
	if d.Min != 0 || d.Max != 3 {
		t.Fatalf("KArrange Min/Max = %d/%d, want 0/3 (Surveil 1 + Enhanced Surveillance's 2)", d.Min, d.Max)
	}
	if len(d.Options) != 3 {
		t.Fatalf("KArrange options = %d, want 3", len(d.Options))
	}
	for i, o := range d.Options {
		if o.Kind != "graveyard" {
			t.Fatalf("option %d Kind = %q, want \"graveyard\" (a Surveil)", i, o.Kind)
		}
	}
	replayCheck(t, e, cfg)
}

// TestEnhancedSurveillanceDeclinedKeepsTheBaseCount proves the Optional$
// election is real, not decorative: answering "no" leaves the Surveil 1 at
// its own one-card count.
func TestEnhancedSurveillanceDeclinedKeepsTheBaseCount(t *testing.T) {
	e, _, id, esID := surveilNumFixture(t, 602)
	if o := e.G.Obj(esID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Enhanced Surveillance %v not on the battlefield: %+v", esID, e.G.Obj(esID))
	}
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the may-look election, got %+v", d)
	}
	submitChoices(t, e, 1) // no: keep the base count
	d = e.Pending()
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the KArrange after declining, got %+v", d)
	}
	if d.Max != 1 {
		t.Fatalf("KArrange Max = %d, want 1 (declined, so the base Surveil 1)", d.Max)
	}
}

// TestSurveilWithoutSurveilNumStaticLooksAtOne is the control: the very same
// Surveil 1 with no static out poses no election and looks at exactly one
// card, so the +2 above is the static's doing and not a general rig.
func TestSurveilWithoutSurveilNumStaticLooksAtOne(t *testing.T) {
	e, cfg, id := newFixtureDeck(t, 603, surveilOneSrc)
	addMana(t, e, 0, "U")
	d := surveilDecision(t, e, id)
	if d.Max != 1 {
		t.Fatalf("KArrange Max without the static = %d, want 1", d.Max)
	}
	replayCheck(t, e, cfg)
}

// TestSurveilNumStaticIsRegistered pins the census: effects.Supported reports
// the mode, so a future deck (the ratchet's census card) can rely on it.
func TestSurveilNumStaticIsRegistered(t *testing.T) {
	if !effects.Supported()["stat:SurveilNum"] {
		t.Fatal("stat:SurveilNum not reported by effects.Supported()")
	}
}

// surveilNumMandatorySrc is a freely-authored MANDATORY stat:SurveilNum
// static (no Optional$): the arm of the mode the sole corpus carrier does not
// exercise, pinned here so extraOf's mandatory path has its own defence.
const surveilNumMandatorySrc = "Name:Mandatory Surveillance\nManaCost:U\n" +
	"Types:Enchantment\nS:Mode$ SurveilNum | ValidPlayer$ You | Num$ 2\nOracle:x\n"

// mandatoryStaticFixture seats the authored mandatory static on seat 0's
// battlefield and the authored Surveil 1 in its hand (no corpus dependency).
func mandatoryStaticFixture(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	stat := card(t, surveilNumMandatorySrc)
	src := card(t, surveilOneSrc)
	name := stat.Faces[0].Name
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Tokens: map[string]*cards.Card{},
		Decks: [][]*cards.Card{
			append([]*cards.Card{stat, src}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	moveByName(t, e, 0, name, state.ZBattlefield)
	id := moveSurveilToHand(t, e, 0, "Surveil One")
	addMana(t, e, 0, "U")
	return e, cfg, id
}

// countArrangeAsks counts the DecisionAsk events of the given decision kinds
// after index mark -- the "exactly one election, exactly one arrange" probe
// the pre-fix ping-pong failed (it re-posed both every round).
func countArrangeAsks(e *Engine, mark int, kinds ...decision.Kind) int {
	want := make(map[decision.Kind]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	n := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.DecisionAsk && want[decision.Kind(ev.Text)] {
			n++
		}
	}
	return n
}

// TestEnhancedSurveillanceAcceptedArrangeCompletesOnce answers BOTH asks --
// the may-look election AND the KArrange -- and proves the resolution
// terminates: exactly one election and exactly one arrange are ever posed,
// and the arrangement applies for real once (the unpicked card to the
// graveyard, the picked ones on top in answer order). Before the arrange
// re-entry gate the two asks ping-ponged forever, each round applying the
// arrange again and draining the library into the graveyard.
func TestEnhancedSurveillanceAcceptedArrangeCompletesOnce(t *testing.T) {
	e, cfg, id, esID := surveilNumFixture(t, 604)
	if o := e.G.Obj(esID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Enhanced Surveillance %v not on seat 0's battlefield: %+v", esID, e.G.Obj(esID))
	}
	if n := len(e.G.Zone(state.ZLibrary, 0)); n < 5 {
		t.Fatalf("precondition: seat 0 library has %d cards, need >= 5 so the remainder beneath the looked-at three is non-empty", n)
	}
	mark := len(e.L.Events)
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the may-look election first, got %+v", d)
	}
	submitChoices(t, e, 0) // yes: look at the additional cards
	d = e.Pending()
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the KArrange after accepting, got %+v", d)
	}
	if d.Max != 3 || len(d.Options) != 3 {
		t.Fatalf("KArrange Max/options = %d/%d, want 3/3 (Surveil 1 + the static's 2)", d.Max, len(d.Options))
	}
	picked := [2]state.ObjID{d.Options[2].Obj, d.Options[0].Obj}
	toGrave := d.Options[1].Obj
	libBefore := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	submitChoices(t, e, 2, 0) // keep options 2 then 0 on top; option 1 to the graveyard

	if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after answering the arrange, pending = %+v: the resolution must complete (priority next), never re-ask", d)
	}
	if n := countArrangeAsks(e, mark, decision.KChoose, decision.KArrange); n != 2 {
		t.Fatalf("election+arrange asks since the cast = %d, want exactly 2 (1+1): the pre-fix ping-pong re-posed both every round", n)
	}
	libAfter := e.G.Zone(state.ZLibrary, 0)
	if libAfter[0] != picked[0] || libAfter[1] != picked[1] {
		t.Fatalf("library top = %v,%v, want the answer order %v,%v", libAfter[0], libAfter[1], picked[0], picked[1])
	}
	if len(libAfter) != len(libBefore)-1 {
		t.Fatalf("library size %d, want %d (exactly the one unpicked card left it)", len(libAfter), len(libBefore)-1)
	}
	for i := 2; i < len(libAfter); i++ {
		if libAfter[i] != libBefore[i+1] {
			t.Fatalf("library[%d] = %v, want %v: the untouched remainder must sit beneath the picked pair", i, libAfter[i], libBefore[i+1])
		}
	}
	found := false
	for _, gid := range e.G.Zone(state.ZGraveyard, 0) {
		if gid == toGrave {
			found = true
		}
	}
	if !found {
		t.Fatalf("unpicked card %v not in seat 0's graveyard: %v", toGrave, e.G.Zone(state.ZGraveyard, 0))
	}
	replayCheck(t, e, cfg)
}

// TestEnhancedSurveillanceDeclinedArrangeCompletes answers the election "no"
// AND the KArrange: the base one-card surveil completes (priority next),
// again with exactly one of each ask and no library->graveyard move.
func TestEnhancedSurveillanceDeclinedArrangeCompletes(t *testing.T) {
	e, cfg, id, _ := surveilNumFixture(t, 605)
	mark := len(e.L.Events)
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the may-look election first, got %+v", d)
	}
	submitChoices(t, e, 1) // no: keep the base count
	d = e.Pending()
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the KArrange after declining, got %+v", d)
	}
	if d.Max != 1 || len(d.Options) != 1 {
		t.Fatalf("KArrange Max/options = %d/%d, want 1/1 (declined, so the base Surveil 1)", d.Max, len(d.Options))
	}
	libBefore := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	submitChoices(t, e, 0) // keep the one looked-at card on top
	if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after answering the arrange, pending = %+v: the resolution must complete (priority next), never re-ask", d)
	}
	if n := countArrangeAsks(e, mark, decision.KChoose, decision.KArrange); n != 2 {
		t.Fatalf("election+arrange asks since the cast = %d, want exactly 2 (1+1)", n)
	}
	libAfter := e.G.Zone(state.ZLibrary, 0)
	if len(libAfter) != len(libBefore) {
		t.Fatalf("library size %d, want %d: a declined extra look with the card kept moves nothing", len(libAfter), len(libBefore))
	}
	for i := range libAfter {
		if libAfter[i] != libBefore[i] {
			t.Fatalf("library[%d] = %v, want %v (order unchanged)", i, libAfter[i], libBefore[i])
		}
	}
	replayCheck(t, e, cfg)
}

// TestSurveilNumMandatoryStaticAddsCountWithoutAsking pins the mandatory arm:
// an authored Optional-absent stat:SurveilNum static raises the same Surveil 1
// to a three-card look with NO election posed at all, and the answered
// KArrange completes the resolution.
func TestSurveilNumMandatoryStaticAddsCountWithoutAsking(t *testing.T) {
	e, cfg, id := mandatoryStaticFixture(t, 606)
	mark := len(e.L.Events)
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the KArrange directly (mandatory adds, no election), got %+v", d)
	}
	if d.Max != 3 || len(d.Options) != 3 {
		t.Fatalf("KArrange Max/options = %d/%d, want 3/3 (Surveil 1 + the mandatory 2)", d.Max, len(d.Options))
	}
	submitChoices(t, e, 1, 0) // keep options 1 then 0 on top; option 2 to the graveyard
	if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after answering the arrange, pending = %+v: the resolution must complete (priority next)", d)
	}
	if n := countArrangeAsks(e, mark, decision.KChoose); n != 0 {
		t.Fatalf("election asks since the cast = %d, want 0: a mandatory static must not ask", n)
	}
	if n := countArrangeAsks(e, mark, decision.KArrange); n != 1 {
		t.Fatalf("arrange asks since the cast = %d, want exactly 1", n)
	}
	replayCheck(t, e, cfg)
}

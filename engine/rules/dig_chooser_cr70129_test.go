package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cr-g08: a Dig over ANOTHER player's library (Jace, the Mind Sculptor's
// [+2] on an opponent; fateseal, CR 701.29a) is chosen by the resolving
// ability's CONTROLLER, not the library's owner, unless the card names a
// Choser$. The private look goes to that chooser, and fateseal's "the rest
// on top of that library in any order" is a real order the chooser picks.

// passToPendingResume passes priority until the pending decision is a
// mid-resolution ask with the given ResumeKind, fatal on any other
// non-priority decision or when the stack empties first.
func passToPendingResume(t *testing.T, e *Engine, kind decision.Kind, resume string, limit int) *decision.Decision {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending while waiting for %q", resume)
		}
		if d.Kind == kind && d.ResumeKind == resume {
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision while waiting for %q: kind %v resume %q player %d prompt %q",
				resume, d.Kind, d.ResumeKind, d.Player, d.Prompt)
		}
		if len(e.G.Stack) == 0 {
			t.Fatalf("stack emptied before the %q ask was posed", resume)
		}
		passOnce(t, e)
	}
	t.Fatalf("no %q ask within %d passes", resume, limit)
	return nil
}

// TestJacePlusTwoOnOpponentIsChosenByJacesController: Jace's [+2] targeting
// seat 1 poses its "you may put that card on the bottom" ask to seat 0
// (Jace's controller), the private look carrying seat 1's top card goes to
// seat 0 and NOT to seat 1, and the answer puts the card on the bottom of
// seat 1's library.
func TestJacePlusTwoOnOpponentIsChosenByJacesController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	jaceCard := mustCorpusCard(t, reg, "Jace, the Mind Sculptor")
	plusTwo := jaceAbility(t, jaceCard, "AddCounter", 2)
	e, cfg, jace := walkerBoard(t, reg, "Jace, the Mind Sculptor")
	e.Advance()
	oppTop := e.G.Zone(state.ZLibrary, 1)[0]
	submitChoices(t, e, abilityOption(t, e, jace, plusTwo).Index)
	targetPlayer(t, e, 1)
	mark := len(e.L.Events)
	d := passToPendingResume(t, e, decision.KChoose, "dig", 20)
	if d.Player != 0 {
		t.Fatalf("Jace's [+2] dig ask posed to player %d, want 0 (Jace's controller; CR 701.29a / 'you may put')", d.Player)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != oppTop {
		t.Fatalf("dig options = %+v, want the one card on top of seat 1's library (%v)", d.Options, oppTop)
	}
	var lookTo []state.PlayerID
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && ev.Secret && ev.Text == "looks at the top of the library" {
			lookTo = append(lookTo, ev.Player)
			if len(ev.IDs) != 1 || ev.IDs[0] != oppTop {
				t.Fatalf("look Note ids = %v, want [%v]", ev.IDs, oppTop)
			}
		}
	}
	if len(lookTo) != 1 || lookTo[0] != 0 {
		t.Fatalf("private look went to %v, want exactly [0] (the chooser; the owner does not see an unrevealed look)", lookTo)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	lib := e.G.Zone(state.ZLibrary, 1)
	if lib[len(lib)-1] != oppTop {
		t.Fatalf("seat 1's library bottom = %v, want the chosen card %v", lib[len(lib)-1], oppTop)
	}
	replayCheck(t, e, cfg)
}

// TestSpinIntoMythFatesealCasterChoosesAndOrdersTheTop: Spin into Myth's
// fateseal 2 on seat 1 is chosen by the caster (seat 0); keeping both cards
// on top poses a second ask -- the order of the rest on top (CR 701.29a "the
// rest on top of that library in any order") -- also to seat 0, and the
// answered order is the new top of seat 1's library.
func TestSpinIntoMythFatesealCasterChoosesAndOrdersTheTop(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	spin := lookup(t, reg, "Spin into Myth")
	bear := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{spin, bear}, nil)
	moveByName(t, e, 0, "Spin into Myth", state.ZHand)
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	addMana(t, e, 0, "UUUUU")
	top := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 1)[:2]...)
	castNamed(t, e, "Spin into Myth")
	targetObject(t, e, bearID)
	// The ChoosePlayer step (Defined$ You | Choices$ Player.Opponent) has one
	// legal answer at a two-seat table; answer it if the engine asks.
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil || d.ResumeKind == "dig" {
			break
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "dig" {
		t.Fatalf("no fateseal dig ask pending: %+v", d)
	}
	if d.Player != 0 {
		t.Fatalf("fateseal ask posed to player %d, want 0 (the caster; CR 701.29a)", d.Player)
	}
	// Keep both on top: the empty answer (ChangeNum$ Any, Min 0).
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("submit keep-both: %v", err)
	}
	a := e.Pending()
	if a == nil || a.Kind != decision.KArrange || a.ResumeKind != "dig_arrange" {
		t.Fatalf("no top-order arrange after keeping both cards on top (CR 701.29a 'the rest on top ... in any order'): %+v", a)
	}
	if a.Player != 0 || len(a.Options) != 2 || a.Min != 2 || a.Max != 2 {
		t.Fatalf("top-order arrange player/options/min/max = %d/%d/%d/%d, want 0/2/2/2", a.Player, len(a.Options), a.Min, a.Max)
	}
	// Answer the reverse of the offered order.
	submitChoices(t, e, a.Options[1].Index, a.Options[0].Index)
	passUntilStackEmpty(t, e, 20)
	lib := e.G.Zone(state.ZLibrary, 1)
	if lib[0] != top[1] || lib[1] != top[0] {
		t.Fatalf("seat 1's top two = %v, want the answered order %v", lib[:2], []state.ObjID{top[1], top[0]})
	}
	replayCheck(t, e, cfg)
}

// TestJacePlusTwoDeclineLeavesTheCardOnTop: declining Jace's [+2] ("you
// MAY put that card on the bottom") leaves seat 1's top card on top -- the
// script's LibraryPosition2$ 0 with no DestinationZone2$ keeps the untaken
// card where it is.
func TestJacePlusTwoDeclineLeavesTheCardOnTop(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	jaceCard := mustCorpusCard(t, reg, "Jace, the Mind Sculptor")
	plusTwo := jaceAbility(t, jaceCard, "AddCounter", 2)
	e, cfg, jace := walkerBoard(t, reg, "Jace, the Mind Sculptor")
	e.Advance()
	before := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 1)...)
	submitChoices(t, e, abilityOption(t, e, jace, plusTwo).Index)
	targetPlayer(t, e, 1)
	d := passToPendingResume(t, e, decision.KChoose, "dig", 20)
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("submit decline: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
	if lib := e.G.Zone(state.ZLibrary, 1); !slices.Equal(lib, before) {
		t.Fatalf("seat 1's library = %v, want %v unchanged (declined: the card stays on top)", lib, before)
	}
	replayCheck(t, e, cfg)
}

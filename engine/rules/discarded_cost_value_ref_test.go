package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDiscardedCostValueRef casts the real corpus Grab the Prize paying its
// "Discard<1/Card>" additional cost with a nonland and then a land card. The
// damage SVar reads `Discarded$Valid Card.nonLand/Times.2`, so the nonland
// discard deals 2 to the opponent and the land discard deals none; both draw
// two.
func TestDiscardedCostValueRef(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		discard string
		damage  int32
	}{
		{"nonland", "Grizzly Bears", 2},
		{"land", "Forest", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := testutil.CorpusRegistry(t)
			e, cfg := mordorEngine(t, reg, 4201, "Grab the Prize", "Grizzly Bears", "Forest")
			grab := searchMoveByName(t, e, "Grab the Prize", state.ZHand)
			disc := searchMoveByName(t, e, tc.discard, state.ZHand)
			addMana(t, e, 0, "1R")
			life, draws := e.G.Players[1].Life, countDrawFor(e, 0)
			castNoDrain(t, e, grab)
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "discard" {
				t.Fatalf("cost discard ask = %+v", d)
			}
			submitObj(t, e, "discard", disc)
			passUntilStackEmpty(t, e, 30)
			if o := e.G.Obj(disc); o == nil || o.Zone != state.ZGraveyard {
				t.Fatalf("discarded card not in the graveyard: %+v", o)
			}
			if got := countDrawFor(e, 0) - draws; got != 2 {
				t.Fatalf("drew %d, want 2", got)
			}
			if got := life - e.G.Players[1].Life; got != tc.damage {
				t.Fatalf("opponent lost %d life, want %d", got, tc.damage)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestDiscardedCostValueRefSurvivesResponseSpell is counter-scenario (a) from
// the review: another spell put on the stack between Grab the Prize's cost
// discard and its resolution truncates the cost-move LOG WINDOW, so a value
// read that scans that window loses the discard. The paid list is keyed by
// the stack object, so X still counts the nonland Grizzly Bears and the spell
// still deals 2.
func TestDiscardedCostValueRefSurvivesResponseSpell(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := mordorEngine(t, reg, 4301, "Grab the Prize", "Grizzly Bears", "Dark Ritual")
	grab := searchMoveByName(t, e, "Grab the Prize", state.ZHand)
	disc := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	ritual := searchMoveByName(t, e, "Dark Ritual", state.ZHand)
	addMana(t, e, 0, "1RB")
	life, draws := e.G.Players[1].Life, countDrawFor(e, 0)
	castNoDrain(t, e, grab)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "discard" {
		t.Fatalf("cost discard ask = %+v", d)
	}
	submitObj(t, e, "discard", disc)
	// Respond with Dark Ritual: its PutOnStack used to be the window boundary
	// that erased the discard; the window is now Grab's own cast, so the
	// discard stays readable through the log window as well.
	castNoDrain(t, e, ritual)
	if got := e.CostMovesInWindow(grab, events.CostMoveDiscard); len(got) != 1 || got[0] != disc {
		t.Fatalf("precondition: response spell hid the cost discard from the window: %v, want [%d]", got, disc)
	}
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(ritual); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Dark Ritual did not resolve: %+v", o)
	}
	if o := e.G.Obj(disc); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("discarded card not in the graveyard: %+v", o)
	}
	if got := countDrawFor(e, 0) - draws; got != 2 {
		t.Fatalf("drew %d, want 2", got)
	}
	if got := life - e.G.Players[1].Life; got != 2 {
		t.Fatalf("opponent lost %d life, want 2", got)
	}
	replayCheck(t, e, cfg)
}

// TestDiscardedCostValueRefIgnoresResponseManaDiscard is counter-scenario (b):
// a mana ability has no stack push, so a discard paid to activate one in
// response sits inside the cost-move LOG WINDOW. The paid list belongs to
// Grab the Prize alone, so discarding a nonland to Skirge Familiar after
// paying Grab's cost with a land still leaves X at 0 and deals no damage.
func TestDiscardedCostValueRefIgnoresResponseManaDiscard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := mordorEngine(t, reg, 4302, "Grab the Prize", "Forest", "Grizzly Bears", "Skirge Familiar")
	grab := searchMoveByName(t, e, "Grab the Prize", state.ZHand)
	land := searchMoveByName(t, e, "Forest", state.ZHand)
	nonland := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	skirge := mordorMove(t, e, 0, "Skirge Familiar", state.ZBattlefield)
	addMana(t, e, 0, "1R")
	life, draws := e.G.Players[1].Life, countDrawFor(e, 0)
	castNoDrain(t, e, grab)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "discard" {
		t.Fatalf("cost discard ask = %+v", d)
	}
	submitObj(t, e, "discard", land)
	// Respond by activating Skirge Familiar's mana ability, discarding a
	// nonland. A mana ability pushes nothing, so this discard joins the
	// cost-move window the old value read scanned.
	activateMana(t, e, skirge)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "mana_discard" {
		t.Fatalf("Skirge Familiar discard ask = %+v", d)
	}
	submitObj(t, e, "mana_discard", nonland)
	if got := e.CostMovesInWindow(grab, events.CostMoveDiscard); len(got) != 2 {
		t.Fatalf("precondition: foreign mana-ability discard not in the cost-move window: %v", got)
	}
	passUntilStackEmpty(t, e, 30)
	for _, id := range []state.ObjID{land, nonland} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: discarded card %d not in the graveyard: %+v", id, o)
		}
	}
	if got := countDrawFor(e, 0) - draws; got != 2 {
		t.Fatalf("drew %d, want 2", got)
	}
	if got := life - e.G.Players[1].Life; got != 0 {
		t.Fatalf("opponent lost %d life, want 0 (foreign discard must not count)", got)
	}
	replayCheck(t, e, cfg)
}

package rules

// CR 702.148a: "Cleave [cost]" means "You may cast this spell by paying
// [cost] rather than paying its mana cost" and "If this spell's cleave cost
// was paid, change its text by removing all text found within square
// brackets". Forge scripts the cleave cast as a second SP ability carrying
// NonBasicSpell$ True whose parameters are the bracket-free text; the
// cleaved cast pays that ability's Cost$ instead of the mana cost and
// resolves (targets included) with that ability.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cleaveOption returns the pending "cast" option for id with Mode mode.
func cleaveOption(e *Engine, id state.ObjID, mode string) (decision.Option, bool) {
	d := e.Pending()
	if d == nil {
		return decision.Option{}, false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			return o, true
		}
	}
	return decision.Option{}, false
}

// TestCR702_148aPathOfPerilCleaveDestroysEveryCreature: Path of Peril
// ("Destroy all creatures [with mana value 2 or less]", cleave {4}{W}{B}) is
// offered for its cleave cost and, cleaved, destroys Hill Giant (MV 4) as
// well as Grizzly Bears.
func TestCR702_148aPathOfPerilCleaveDestroysEveryCreature(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 7021, map[string]state.Zone{
		"Path of Peril": state.ZHand, "Hill Giant": state.ZBattlefield, "Grizzly Bears": state.ZBattlefield})
	path, giant, bears := ids["Path of Peril"], ids["Hill Giant"], ids["Grizzly Bears"]
	addMana(t, e, 0, "WBCCCC")
	e.priorityRound()
	opt, ok := cleaveOption(e, path, "cleaved")
	if !ok {
		t.Fatalf("CR 702.148a Path of Peril seq %d: no cleave cast offered with {4}{W}{B} floating", len(e.L.Events))
	}
	submitChoices(t, e, opt.Index)
	for i := 0; i < 8 && len(e.G.Stack) == 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) != 1 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("CR 702.148a Path of Peril seq %d: stack %d, pool %v after the cleave cast, want the spell on the stack and {4}{W}{B} spent", len(e.L.Events), len(e.G.Stack), e.G.Players[0].Pool)
	}
	passToResolution(t, e)
	if z := e.G.Obj(giant).Zone; z != state.ZGraveyard {
		t.Errorf("CR 702.148a Path of Peril seq %d: Hill Giant (MV 4) zone = %s, want Graveyard (the bracketed MV restriction is removed)", len(e.L.Events), z)
	}
	if z := e.G.Obj(bears).Zone; z != state.ZGraveyard {
		t.Errorf("CR 702.148a Path of Peril seq %d: Grizzly Bears zone = %s, want Graveyard", len(e.L.Events), z)
	}
	replayCheck(t, e, cfg)
}

// TestCR702_148aPathOfPerilPlainCastKeepsTheBrackets guards the plain cast:
// {1}{B}{B} destroys only the MV 2 or less creature.
func TestCR702_148aPathOfPerilPlainCastKeepsTheBrackets(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 7022, map[string]state.Zone{
		"Path of Peril": state.ZHand, "Hill Giant": state.ZBattlefield, "Grizzly Bears": state.ZBattlefield})
	path, giant, bears := ids["Path of Peril"], ids["Hill Giant"], ids["Grizzly Bears"]
	addMana(t, e, 0, "BBC")
	e.priorityRound()
	if _, ok := cleaveOption(e, path, "cleaved"); ok {
		t.Fatalf("CR 702.148a Path of Peril seq %d: cleave cast offered with only {1}{B}{B} floating", len(e.L.Events))
	}
	opt, ok := cleaveOption(e, path, "")
	if !ok {
		t.Fatalf("CR 702.148a Path of Peril seq %d: plain cast not offered", len(e.L.Events))
	}
	submitChoices(t, e, opt.Index)
	passToResolution(t, e)
	if z := e.G.Obj(bears).Zone; z != state.ZGraveyard {
		t.Errorf("CR 702.148a Path of Peril seq %d: Grizzly Bears zone = %s, want Graveyard", len(e.L.Events), z)
	}
	if z := e.G.Obj(giant).Zone; z != state.ZBattlefield {
		t.Errorf("CR 702.148a Path of Peril seq %d: Hill Giant zone = %s, want Battlefield (MV 4 survives the plain cast)", len(e.L.Events), z)
	}
	replayCheck(t, e, cfg)
}

// TestCR702_148aLunarRejectionCleaveTargetsAnyCreature: Lunar Rejection
// ("Return target [Wolf or Werewolf] creature to its owner's hand. Draw a
// card.", cleave {3}{U}) cleaved may target Grizzly Bears, which the plain
// cast cannot, and resolves with the cleave ability's own target spec.
func TestCR702_148aLunarRejectionCleaveTargetsAnyCreature(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 7023, map[string]state.Zone{
		"Lunar Rejection": state.ZHand, "Grizzly Bears": state.ZBattlefield})
	spell, bears := ids["Lunar Rejection"], ids["Grizzly Bears"]
	addMana(t, e, 0, "UCCC")
	e.priorityRound()
	if _, ok := cleaveOption(e, spell, ""); ok {
		t.Fatalf("CR 702.148a Lunar Rejection seq %d: plain cast offered with no Wolf or Werewolf to target", len(e.L.Events))
	}
	opt, ok := cleaveOption(e, spell, "cleaved")
	if !ok {
		t.Fatalf("CR 702.148a Lunar Rejection seq %d: no cleave cast offered with {3}{U} floating and a creature to target", len(e.L.Events))
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("CR 702.148a Lunar Rejection seq %d: decision %+v, want the target ask", len(e.L.Events), d)
	}
	pick := -1
	for i, o := range d.Options {
		if o.Obj == bears {
			pick = i
		}
	}
	if pick < 0 {
		t.Fatalf("CR 702.148a Lunar Rejection seq %d: Grizzly Bears not a legal cleaved target", len(e.L.Events))
	}
	submitChoices(t, e, d.Options[pick].Index)
	passToResolution(t, e)
	if z := e.G.Obj(bears).Zone; z != state.ZHand {
		t.Errorf("CR 702.148a Lunar Rejection seq %d: Grizzly Bears zone = %s, want Hand", len(e.L.Events), z)
	}
	// Lunar Rejection left the hand, Grizzly Bears returned, and a card was drawn.
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Errorf("CR 702.148a Lunar Rejection seq %d: hand %d, want %d (cast -1, bounce +1, draw +1)", len(e.L.Events), got, hand+1)
	}
	replayCheck(t, e, cfg)
}

// TestCR702_148aDigUpCleaveSearchesForAnyCard: Dig Up ("Search your library
// for a [basic land] card, [reveal it,] put it into your hand", cleave
// {1}{B}{B}{G}) cleaved finds a nonland card through its mid-resolution
// search ask, which resumes with the cleave ability.
func TestCR702_148aDigUpCleaveSearchesForAnyCard(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 7024, map[string]state.Zone{
		"Dig Up": state.ZHand, "Grizzly Bears": state.ZLibrary})
	dig, bears := ids["Dig Up"], ids["Grizzly Bears"]
	addMana(t, e, 0, "BBGC")
	e.priorityRound()
	opt, ok := cleaveOption(e, dig, "cleaved")
	if !ok {
		t.Fatalf("CR 702.148a Dig Up seq %d: no cleave cast offered with {1}{B}{B}{G} floating", len(e.L.Events))
	}
	submitChoices(t, e, opt.Index)
	passToResolution(t, e)
	d := e.Pending()
	if d == nil || d.Kind == decision.KPriority {
		t.Fatalf("CR 702.148a Dig Up seq %d: no search ask after resolution began (pending %+v)", len(e.L.Events), d)
	}
	pick := -1
	for i, o := range d.Options {
		if o.Obj == bears {
			pick = i
		}
	}
	if pick < 0 {
		t.Fatalf("CR 702.148a Dig Up seq %d: Grizzly Bears (a nonland card) not offered by the cleaved search", len(e.L.Events))
	}
	submitChoices(t, e, d.Options[pick].Index)
	for i := 0; i < 8; i++ {
		if d := e.Pending(); d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, e.Pending().Options[0].Index)
	}
	if z := e.G.Obj(bears).Zone; z != state.ZHand {
		t.Errorf("CR 702.148a Dig Up seq %d: Grizzly Bears zone = %s, want Hand", len(e.L.Events), z)
	}
	replayCheck(t, e, cfg)
}

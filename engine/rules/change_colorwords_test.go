package rules

// Swirl the Mists (CR 612.5): "As Swirl the Mists enters, choose a color.
// All instances of color words in the text of spells and permanents become
// the chosen color word." The as-enters choice already works (the
// K:ETBReplacement ChooseColor repl); this pins the reader of the card's
// Continuous static (Affected$ Spell,Permanent | AffectedZone$ Stack,
// Battlefield | ChangeColorWordsTo$ ChosenColor): while it is live, a
// colour word in an affected spell's or permanent's text reads as the
// chosen colour -- in its TARGETING, not just its rendered text (Terror's
// ValidTgts$ Creature.nonArtifact+nonBlack reads nonRed; Northern
// Paladin's ValidTgts$ Permanent.Black reads Permanent.Red), and the
// change ends when Swirl leaves (the static stops, the substitutions
// retract).
//
// Fixture notes: Bloodthrone Vampire is the black creature (no ETB
// trigger), Raging Goblin the red one, Mountain the red noncreature
// permanent (Northern Paladin's target), Naturalize the colour-word-free
// removal for Swirl itself (Terror targets creatures only, so it cannot
// remove the enchantment whose change reverted the words).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestChangeColorWords(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{
		lookup(t, reg, "Swirl the Mists"),
		lookup(t, reg, "Terror"), lookup(t, reg, "Terror"),
		lookup(t, reg, "Naturalize"),
		lookup(t, reg, "Northern Paladin"),
		lookup(t, reg, "Bloodthrone Vampire"),
		lookup(t, reg, "Raging Goblin"),
	}, nil)

	// Precondition: the two colour carriers sit on the battlefield and their
	// printed colours differ, so the substitution is observable in both
	// directions.
	black := moveCorpusCard(t, e, "Bloodthrone Vampire", 0, state.ZBattlefield)
	red := moveCorpusCard(t, e, "Raging Goblin", 0, state.ZBattlefield)
	terror1 := moveCorpusCard(t, e, "Terror", 0, state.ZHand)
	moveCorpusCard(t, e, "Naturalize", 0, state.ZHand)
	var paladin state.ObjID
	if black == 0 || red == 0 || terror1 == 0 {
		t.Fatal("precondition: fixtures not placed")
	}
	if !strings.ContainsRune(e.Colors(black), 'B') || !strings.ContainsRune(e.Colors(red), 'R') {
		t.Fatalf("precondition: colours wrong: black %q red %q", e.Colors(black), e.Colors(red))
	}

	// The Paladin is a battlefield permanent before Swirl enters, so the
	// static reaches its text where it lives.
	addMana(t, e, 0, "WWWW")
	moveCorpusCard(t, e, "Northern Paladin", 0, state.ZHand)
	castNamed(t, e, "Northern Paladin")
	passUntilStackEmpty(t, e, 40)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == "Northern Paladin" {
			paladin = id
		}
	}
	if o := e.G.Obj(paladin); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the Paladin is not on the battlefield: %+v", o)
	}

	// Swirl enters and its as-enters choice records Red.
	addMana(t, e, 0, "CCUUU")
	swirl := castCorpusCardToBattlefield(t, e, 0, "Swirl the Mists", 3) // Red
	if o := e.G.Obj(swirl); o == nil || o.Zone != state.ZBattlefield || o.ChosenColor != "R" {
		t.Fatalf("precondition: Swirl not on the battlefield with Red chosen: %+v", o)
	}

	// The static is live: the battlefield Paladin's text reads "red" where
	// it printed "black".
	if d := e.Derived(paladin); !strings.Contains(strings.ToLower(d.Text), "red") ||
		strings.Contains(strings.ToLower(d.Text), "black") {
		t.Fatalf("Paladin derived text = %q, want the colour word changed to red", d.Text)
	}

	// A spell's text changes while it sits on the stack: Terror's
	// nonBlack clause reads nonRed, so the black creature is a legal target
	// and the red one is not.
	addMana(t, e, 0, "CCB")
	e.pending = nil
	e.Advance()
	castNamed(t, e, "Terror")
	d := passUntilAskKind(t, e, "target", 40)
	if d == nil || len(d.Options) == 0 {
		t.Fatalf("no target ask for Terror: %+v", d)
	}
	refs := targetRefs(t, e, d)
	if !strings.Contains(refs, "Bloodthrone Vampire") {
		t.Fatalf("target options %v, want the black creature legal (nonblack reads nonred)", d.Options)
	}
	if strings.Contains(refs, "Raging Goblin") {
		t.Fatalf("target options %v, the red creature must be illegal (nonblack reads nonred)", d.Options)
	}
	submitChoices(t, e, optionIndexOf(t, d, "Bloodthrone Vampire"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(black); o == nil || o.Zone == state.ZBattlefield {
		t.Fatalf("Terror did not destroy the black creature: %+v", o)
	}

	// A permanent's text changes the same way: Northern Paladin's
	// "Destroy target black permanent" reads red, so the red Goblin is a
	// legal target and the black Vampire is not. The ability costs {T}, so
	// the activation waits for the Paladin's own second turn (summoning
	// sickness, CR 302.6).
	passUntilTurn(t, e, 3)
	addMana(t, e, 0, "WWWWW")
	e.pending = nil
	e.Advance()
	d = e.Pending()
	idx := -1
	for _, o := range d.Options {
		if (o.Kind == "ability" || o.Kind == "activate") && o.Obj == paladin {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no Paladin ability option in %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = passUntilAskKind(t, e, "target", 40)
	if d == nil {
		t.Fatalf("no target ask for the Paladin ability")
	}
	refs = targetRefs(t, e, d)
	if !strings.Contains(refs, "Raging Goblin") {
		t.Fatalf("target options %v, want the red Goblin legal (black reads red)", d.Options)
	}
	if strings.Contains(refs, "Bloodthrone Vampire") {
		t.Fatalf("target options %v, the black Vampire must be illegal (black reads red)", d.Options)
	}
	submitChoices(t, e, optionIndexOf(t, d, "Raging Goblin"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(red); o == nil || o.Zone == state.ZBattlefield {
		t.Fatalf("the Paladin did not destroy the red Goblin: %+v", o)
	}

	// The change ends when Swirl leaves: the static's substitutions retract.
	nat := moveCorpusCard(t, e, "Naturalize", 0, state.ZHand)
	if nat == 0 {
		t.Fatal("precondition: Naturalize not in hand")
	}
	addMana(t, e, 0, "CG")
	e.pending = nil
	e.Advance()
	castNamed(t, e, "Naturalize")
	d = passUntilAskKind(t, e, "target", 40)
	submitChoices(t, e, optionIndexOf(t, d, "Swirl the Mists"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(swirl); o == nil || o.Zone == state.ZBattlefield {
		t.Fatalf("Naturalize did not remove Swirl: %+v", o)
	}
	for _, id := range []state.ObjID{paladin} {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && len(o.TextChanges) != 0 {
			t.Fatalf("object %d kept its text substitutions after Swirl left: %v", id, o.TextChanges)
		}
	}
	if d := e.Derived(paladin); !strings.Contains(strings.ToLower(d.Text), "black") ||
		strings.Contains(strings.ToLower(d.Text), "red") {
		t.Fatalf("Paladin derived text = %q, want the printed black back", d.Text)
	}
	replayCheck(t, e, cfg)
}

func optionIndexOf(t *testing.T, d *decision.Decision, ref string) int {
	t.Helper()
	for _, o := range d.Options {
		if strings.Contains(o.Label, ref) {
			return o.Index
		}
	}
	t.Fatalf("no option named %q in %+v", ref, d.Options)
	return -1
}

// targetRefs names the target options' objects, so the legality assertions
// read card names the same way the scenario runner's OptionRefs do.
func targetRefs(t *testing.T, e *Engine, d *decision.Decision) string {
	t.Helper()
	var refs []string
	for _, o := range d.Options {
		if o.Obj != 0 {
			if ob := e.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				refs = append(refs, ob.Face().Name)
			}
		}
	}
	return strings.Join(refs, ",")
}

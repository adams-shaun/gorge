package rules

// The AlternativeDecider$ Defined$ grammar (cr-g221). Jailbreak Scheme's
// second Spree mode spells its decider `AlternativeDecider$ ThisTargetedOwner`
// -- the owner of THIS ability's target, the mode's own creature -- while the
// engine used to accept only the literal `TargetedOwner`, emit a Note and take
// the primary (top) placement with no ask. Forge reads the parameter with the
// ordinary Defined$ player grammar (ChangeZoneEffect.java's
// AbilityUtils.getDefinedPlayers), so ThisTargetedOwner is a first-class
// spelling of the same owner referent, not a new vocabulary entry.
//
// Corpus carriers at FORGE_REF 76fd7691ab: `TargetedOwner` 34 (Uncharted
// Voyage and friends, unchanged), `ThisTargetedOwner` 1 (Jailbreak Scheme),
// `RememberedOwner` 1 (Tolarian Contempt). The 19 Standard
// `TargetedOwner` carriers must not change, and the dedicated
// changezone_alternative_decider_test.go run covers them.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// jailbreakSchemeOwnerAnswer casts Jailbreak Scheme with only its {2}
// ChangeZone mode and answers the owner's top/bottom ask with answer. The
// board is a two-seat game: seat 1 owns (and controls) the targeted fixture
// creature, so the ask must go to seat 1, not the caster in seat 0.
func jailbreakSchemeOwnerAnswer(t *testing.T, seed uint64, answer string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	scheme := mustCorpusCard(t, reg, "Jailbreak Scheme")
	bear := card(t, "Name:Jailbreak Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGameSeats(t, seed, []*cards.Card{scheme}, []*cards.Card{bear})

	bearID := moveSeededCard(t, e, 1, bear, state.ZBattlefield)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZBattlefield || o.Owner != 1 || o.Controller != 1 {
		t.Fatalf("precondition: target = %+v, want a battlefield creature owned and controlled by seat 1", o)
	}

	// ManaCost {U} plus the selected mode's ModeCost$ {2}.
	addMana(t, e, 0, "UUU")
	castFixtureNamed(t, e, "Jailbreak Scheme")

	// K:Spree makes the caster announce modes first. Select ONLY the {2}
	// ChangeZone mode so the resolution under test is the owner ask.
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after cast = %+v, want the Spree mode ask", d)
	}
	modeIdx := -1
	for _, o := range d.Options {
		if strings.Contains(o.Label, "top or bottom") {
			modeIdx = o.Index
		}
	}
	if modeIdx < 0 {
		t.Fatalf("Jailbreak Scheme's ChangeZone mode is not offered: %+v", d.Options)
	}
	submitChoices(t, e, modeIdx)

	// CR 601.2c: the announced mode's target is chosen at cast.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after the mode = %+v, want the creature target ask", d)
	}
	targetIdx := -1
	for _, o := range d.Options {
		if o.Obj == bearID {
			targetIdx = o.Index
		}
	}
	if targetIdx < 0 {
		t.Fatalf("precondition: the seeded bear %d is not an offered target: %+v", bearID, d.Options)
	}
	submitChoices(t, e, targetIdx)

	// The owner, not the caster, is asked which library position.
	ask := passPriorityUntil(t, e, decision.KChoose)
	if ask.Player != 1 {
		t.Fatalf("alternative asker = seat %d, want the OWNER seat 1", ask.Player)
	}
	if ask.ResumeKind != "changezone_alternative" {
		t.Fatalf("alternative ask ResumeKind = %q, want changezone_alternative", ask.ResumeKind)
	}
	if len(ask.Options) != 2 {
		t.Fatalf("alternative ask offers %d options, want top/bottom: %+v", len(ask.Options), ask.Options)
	}
	choice := -1
	for _, o := range ask.Options {
		if o.Label == answer {
			choice = o.Index
		}
	}
	if choice < 0 {
		t.Fatalf("answer %q not offered: %+v", answer, ask.Options)
	}
	submitChoices(t, e, choice)
	passUntilStackEmpty(t, e, 20)

	lib := e.G.Zone(state.ZLibrary, 1)
	found := -1
	for i, id := range lib {
		if id == bearID {
			found = i
		}
	}
	if found < 0 {
		t.Fatalf("target %d is not in seat 1's library after the choice: %v", bearID, lib)
	}
	wantPos := len(lib) - 1
	if answer == "top" {
		wantPos = 0
	}
	if found != wantPos {
		t.Fatalf("target library position = %d of %d, want %d (answer %q)", found, len(lib), wantPos, answer)
	}
	// The decider resolved: no loud fallback Note, and the choice was honored.
	if hasNote(e, "cannot resolve an object owner") {
		t.Fatal("ThisTargetedOwner emitted the unresolved-owner Note instead of asking the owner")
	}

	replayCheck(t, e, cfg)
}

func TestJailbreakSchemeOwnerChoosesBottom(t *testing.T) {
	t.Parallel()
	jailbreakSchemeOwnerAnswer(t, 93, "bottom")
}

func TestJailbreakSchemeOwnerChoosesTop(t *testing.T) {
	t.Parallel()
	jailbreakSchemeOwnerAnswer(t, 94, "top")
}

// TestAlternativeDeciderUnknownSpellingStaysLoud pins the fail-closed side of
// the same fix: a spelling the shared Defined$ grammar does not model names
// nobody, so the Note stays and the primary (top) destination is taken.
//
// The brief's example spelling for this case was `Opponent`, but `Opponent`
// is a REAL Defined$ selector (definedSpecCodes): in a two-seat game it names
// exactly one player, so under this fix it is a legal decider and the owner
// ask is posed. The loud fallback therefore has to be tested with a spelling
// the grammar genuinely does not know, which is what this test uses.
func TestAlternativeDeciderUnknownSpellingStaysLoud(t *testing.T) {
	t.Parallel()
	const unknown = "DefinitelyNotADefinedSelector"
	fixture := card(t, "Name:Decider Fixture\nManaCost:U\nTypes:Sorcery\n"+
		"A:SP$ ChangeZone | ValidTgts$ Creature | AlternativeDecider$ "+unknown+
		" | Origin$ Battlefield | Destination$ Library | DestinationAlternative$ Library"+
		" | LibraryPositionAlternative$ -1 | SpellDescription$ Fixture.\n"+
		"Oracle:x\n")
	// The parameter really reached the compiled SA; otherwise the Note below
	// would be absent for the wrong reason.
	if got := fixture.Faces[0].Abilities[0].Params["AlternativeDecider"]; got != unknown {
		t.Fatalf("fixture AlternativeDecider$ = %q, want %q", got, unknown)
	}
	bear := card(t, "Name:Decider Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGameSeats(t, 95, []*cards.Card{fixture}, []*cards.Card{bear})

	bearID := moveSeededCard(t, e, 1, bear, state.ZBattlefield)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZBattlefield || o.Owner != 1 {
		t.Fatalf("precondition: target = %+v, want a battlefield creature owned by seat 1", o)
	}

	addMana(t, e, 0, "U")
	castFixtureNamed(t, e, "Decider Fixture")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("cast = %+v, want the creature target ask", d)
	}
	targetIdx := -1
	for _, o := range d.Options {
		if o.Obj == bearID {
			targetIdx = o.Index
		}
	}
	if targetIdx < 0 {
		t.Fatalf("precondition: the seeded bear %d is not an offered target: %+v", bearID, d.Options)
	}
	submitChoices(t, e, targetIdx)
	passUntilStackEmpty(t, e, 20)

	// No ask was posed; the unknown spelling stayed loud.
	if !hasNote(e, "AlternativeDecider$ "+unknown+" cannot resolve an object owner") {
		t.Fatal("unknown AlternativeDecider$ spelling emitted no unresolved-owner Note")
	}
	lib := e.G.Zone(state.ZLibrary, 1)
	if len(lib) == 0 || lib[0] != bearID {
		t.Fatalf("target was not placed on top of its owner's library: %v", lib)
	}

	replayCheck(t, e, cfg)
}

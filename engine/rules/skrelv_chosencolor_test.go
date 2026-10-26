package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	skrelvBearSrc     = "Name:Skrelv Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	skrelvBlueSrc     = "Name:Skrelv Blue Wall\nManaCost:1 U\nTypes:Creature Wall\nPT:1/1\nOracle:x\n"
	skrelvRedSrc      = "Name:Skrelv Red Wall\nManaCost:1 R\nTypes:Creature Wall\nPT:1/1\nOracle:x\n"
	skrelvBlueSpell   = "Name:Skrelv Blue Source\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n"
	skrelvRedSrcSpell = "Name:Skrelv Red Source\nManaCost:1 R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
)

// TestSkrelvHexproofFromChosenColor drives the real Skrelv, Defector Mite
// ability: choose blue, then the other creature gains toxic 1 and hexproof
// from blue (Pump's DefinedKW$ ChosenColor substitutes the chosen colour in
// KW$) and can't be blocked by blue creatures (the DBEffect's CantBlockBy
// keeps the colour through DBCleanup's ClearChosenColor$).
func TestSkrelvHexproofFromChosenColor(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 77, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Skrelv, Defector Mite"), card(t, skrelvBearSrc)}, mountainDeck(t, 38)...),
			append([]*cards.Card{card(t, skrelvBlueSrc), card(t, skrelvRedSrc), card(t, skrelvBlueSpell), card(t, skrelvRedSrcSpell)}, mountainDeck(t, 36)...),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	skrelv := findAndMoveToHand(t, e, 0, "Skrelv, Defector Mite")
	moveToBattlefield(t, e, skrelv)
	bear := moveSeeded(t, e, 0, skrelvBearSrc, state.ZBattlefield)
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	blueWall := onBoard(t, e, 1, skrelvBlueSrc)
	redWall := onBoard(t, e, 1, skrelvRedSrc)
	blueSrc := onBoard(t, e, 1, skrelvBlueSpell)
	redSrc := onBoard(t, e, 1, skrelvRedSrcSpell)

	// CanBlock only judges an attacking creature: mark the bear as attacking
	// seat 1 (the walls' controller).
	e.G.Obj(bear).IsAttacking = true
	e.G.Obj(bear).Attacking = 1

	// Preconditions: the colours are what the assertions below assume, and
	// the bear has no hexproof, toxic or evasion before the ability.
	if e.Colors(blueWall) != "U" || e.Colors(redWall) != "R" || e.Colors(blueSrc) != "U" || e.Colors(redSrc) != "R" {
		t.Fatalf("precondition: fixture colours = %q %q %q %q", e.Colors(blueWall), e.Colors(redWall), e.Colors(blueSrc), e.Colors(redSrc))
	}
	if e.hexproofBlocksTarget(bear, 1, blueSrc) || e.HasKeyword(bear, "Toxic") {
		t.Fatalf("precondition: bear already hexproof/toxic: %v", e.Keywords(bear))
	}
	if !e.canBlock(blueWall, bear) || !e.canBlock(redWall, bear) {
		t.Fatal("precondition: both walls must be able to block the bear before the ability")
	}

	addMana(t, e, 0, "W")
	opt := abilityOption(t, e, skrelv, 0)
	submitChoices(t, e, opt.Index)
	// {W/P}: pay the white symbol with mana, not life.
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "pay_W" {
		submitChoices(t, e, 0)
	}
	answerKTarget(t, e, bear)
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "choosecolor":
			blue := optionByLabel(d.Options, "Blue")
			if blue < 0 {
				t.Fatalf("choosecolor ask offers no Blue option: %+v", d.Options)
			}
			submitChoices(t, e, blue)
		case d.Kind == decision.KPriority:
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if pass < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, pass)
		default:
			t.Fatalf("unexpected %s decision (resume %q): %+v", d.Kind, d.ResumeKind, d)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not empty after resolution: %d", len(e.G.Stack))
	}
	// The choice was really recorded as blue, then Skrelv's colour changes (a
	// later activation choosing red, or DBCleanup's ClearChosenColor$): the
	// grant and the CantBlockBy effect hold the colour they resolved with.
	if got := e.G.Obj(skrelv).ChosenColor; got != "U" {
		t.Fatalf("Skrelv's recorded choice = %q, want U", got)
	}
	e.G.Obj(skrelv).ChosenColor = "R"

	if !e.HasKeyword(bear, "Toxic") {
		t.Fatalf("bear lacks toxic 1: %v", e.Keywords(bear))
	}
	if !e.hexproofBlocksTarget(bear, 1, blueSrc) {
		t.Fatalf("a blue source can still target the bear; keywords %v", e.Keywords(bear))
	}
	if e.hexproofBlocksTarget(bear, 1, redSrc) {
		t.Fatalf("a red source is withheld from the bear (hexproof from blue only); keywords %v", e.Keywords(bear))
	}
	if e.canBlock(blueWall, bear) {
		t.Fatal("a blue creature can block the bear after Skrelv's ability")
	}
	if !e.canBlock(redWall, bear) {
		t.Fatal("a red creature can't block the bear (only blue creatures are barred)")
	}
}

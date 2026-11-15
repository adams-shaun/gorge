package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const powerstoneTokenSrc = "SP$ Token | ValidTgts$ Opponent | TokenOwner$ %s | TokenTapped$ True | TokenScript$ c_a_powerstone"

func tokenOwnerNoteCount(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unrecognized TokenOwner") {
			n++
		}
	}
	return n
}

// tappedPowerstones counts seat p's tapped Powerstone tokens.
func tappedPowerstones(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o != nil && o.IsToken && o.Face() != nil && strings.Contains(o.Face().Name, "Powerstone") && o.Tapped {
			n++
		}
	}
	return n
}

func resolveOwnerFixture(t *testing.T, seed uint64, owner string) (*Engine, Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		Tokens: reg.Tokens}
	e := New(cfg)
	spell := card(t, "Name:Stone Gift\nTypes:Sorcery\nManaCost:1 R\nA:"+strings.Replace(powerstoneTokenSrc, "%s", owner, 1)+"\nOracle:x\n")
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{IsPlayer: true, Player: 1}}, TargetsOffered: true},
		spell.Faces[0].SpellAbility())
	return e, cfg
}

// TestTokenOwnerTargetedAndYouBothCreate: TokenOwner$ TargetedAndYou mints one
// tapped Powerstone for the controller and one for the targeted opponent.
func TestTokenOwnerTargetedAndYouBothCreate(t *testing.T) {
	t.Parallel()
	e, cfg := resolveOwnerFixture(t, 741, "TargetedAndYou")
	if tokenOwnerNoteCount(e) != 0 {
		t.Fatalf("TargetedAndYou emitted the unrecognized-owner Note")
	}
	for p := state.PlayerID(0); p < 2; p++ {
		if got := tappedPowerstones(e, p); got != 1 {
			t.Fatalf("seat %d tapped Powerstones = %d, want 1 (battlefield %v)", p, got, e.G.Zone(state.ZBattlefield, p))
		}
	}
	replayCheck(t, e, cfg)
}

// TestTokenOwnerUnknownSelectorStillNotes: a spelling nothing knows still
// mints for the controller under the Note.
func TestTokenOwnerUnknownSelectorStillNotes(t *testing.T) {
	t.Parallel()
	e, cfg := resolveOwnerFixture(t, 741, "NoSuchSelector")
	if tokenOwnerNoteCount(e) != 1 {
		t.Fatalf("unknown TokenOwner Notes = %d, want 1", tokenOwnerNoteCount(e))
	}
	if got := tappedPowerstones(e, 0); got != 1 {
		t.Fatalf("controller Powerstones = %d, want 1", got)
	}
	if got := tappedPowerstones(e, 1); got != 0 {
		t.Fatalf("opponent Powerstones = %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestTokenOwnerTargetedAndYouThranSpiderETB: the real card, cast and
// resolved, gives both seats a tapped Powerstone.
func TestTokenOwnerTargetedAndYouThranSpiderETB(t *testing.T) {
	t.Parallel()
	e, cfg, id, caster := corpusCardConfig(t, 741, "Thran Spider")
	opp := 1 - caster
	addMana(t, e, caster, "GGG")
	if tappedPowerstones(e, caster)+tappedPowerstones(e, opp) != 0 {
		t.Fatal("precondition: Powerstones before the cast")
	}
	submitChoices(t, e, castOptionFor(t, e, id).Index)
	for i := 0; i < 40; i++ {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && len(e.G.Stack) == 0 {
			break
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while draining")
		}
		switch d.Kind {
		case decision.KPriority:
			passFirst(t, e)
		case decision.KTarget:
			submitChoices(t, e, 0)
		default:
			t.Fatalf("unexpected decision %q: %+v", d.Kind, d.Options)
		}
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || len(e.G.Stack) != 0 {
		t.Fatalf("precondition: Thran Spider did not resolve: %+v stack %d", o, len(e.G.Stack))
	}
	for _, p := range []state.PlayerID{caster, opp} {
		if got := tappedPowerstones(e, p); got != 1 {
			t.Fatalf("seat %d tapped Powerstones = %d, want 1", p, got)
		}
	}
	if tokenOwnerNoteCount(e) != 0 {
		t.Fatal("Thran Spider emitted the unrecognized-owner Note")
	}
	replayCheck(t, e, cfg)
}

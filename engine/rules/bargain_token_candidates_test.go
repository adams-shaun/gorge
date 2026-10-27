package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.166a: Bargain's sacrifice alternative is "an artifact, an
// enchantment, or a token" -- the token alternative is a TOKEN predicate, not
// a card type. These tests pin that a token which is neither an artifact nor
// an enchantment is a candidate (and that a nontoken creature still is not).

// TestBargainOffersCreatureToken proves the probe board: with only a 1/1
// Human token on the battlefield the bargained mode is offered, the token is
// the sole candidate, answering sacrifices it (it ceases to exist), and the
// resolved Candy Grapple reads as bargained (-5/-5 rather than -3/-3).
func TestBargainOffersCreatureToken(t *testing.T) {
	t.Parallel()
	spell := searchCorpusCard(t, searchTestRegistry(t), "Candy Grapple")
	e, cfg, reg := bargainEngine(t, spell)
	target := moveSeededCard(t, e, 1, searchCorpusCard(t, reg, "Colossal Dreadmaw"), state.ZBattlefield)
	tok := mintToken(t, e, 0, "w_1_1_human")
	// Precondition: the lone battlefield permanent is a token that is neither
	// an artifact nor an enchantment, so it can qualify only through the
	// token predicate. A token that happened to be an artifact would pass the
	// old spec vacuously.
	o := e.G.Obj(tok)
	if o == nil || !o.IsToken || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Human token = %+v, want a battlefield token under seat 0", o)
	}
	if o.Face() == nil || o.Face().IsArtifact() || o.Face().IsEnchantment() {
		t.Fatal("precondition: Human token must be neither an artifact nor an enchantment")
	}
	bf := e.G.Zone(state.ZBattlefield, 0)
	if len(bf) != 1 || bf[0] != tok {
		t.Fatalf("precondition: battlefield = %v, want only the Human token %d", bf, tok)
	}
	if e.Toughness(target) < 6 {
		t.Fatalf("precondition: target toughness %d must survive -5", e.Toughness(target))
	}
	candy := searchMoveByName(t, e, "Candy Grapple", state.ZHand)
	if e.G.Obj(candy).Zone != state.ZHand || !e.HasKeyword(candy, "Bargain") {
		t.Fatal("precondition: Candy Grapple in hand with Bargain")
	}
	addMana(t, e, 0, "BB")
	if !bargainHasCastMode(e, candy, "bargained") {
		t.Fatalf("bargained cast option missing with a creature token: %+v", e.Pending().Options)
	}
	submitChoices(t, e, bargainCastMode(t, e, candy, "bargained").Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 ||
		d.Options[0].Kind != "bargain" || d.Options[0].Obj != tok {
		t.Fatalf("bargain ask = %+v, want the Human token as the only candidate", d)
	}
	bargainAnswer(t, e, tok)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("missing target ask: %+v", d)
	}
	found := false
	for _, opt := range d.Options {
		if opt.Obj == target {
			submitChoices(t, e, opt.Index)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("creature target not offered")
	}
	passUntilStackEmpty(t, e, 60)
	if o := e.G.Obj(tok); o == nil || o.Zone != state.ZCeased {
		t.Fatalf("sacrificed Human token zone = %+v, want ceased (CR 111.7)", o)
	}
	if o := e.G.Obj(target); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("target left the battlefield: %+v", o)
	}
	if got := e.Toughness(target); got != 1 {
		t.Fatalf("target toughness = %d, want 1: the resolved Candy Grapple must read as bargained (-5/-5)", got)
	}
	if got := e.Power(target); got != 1 {
		t.Fatalf("target power = %d, want 1: the resolved Candy Grapple must read as bargained (-5/-5)", got)
	}
	replayCheck(t, e, cfg)
}

// TestBargainStillRefusesNontokenCreature is the negative: a nontoken
// creature alone is not a Bargain candidate, so the bargained mode is not
// offered even though the plain cast is.
func TestBargainStillRefusesNontokenCreature(t *testing.T) {
	t.Parallel()
	spell := searchCorpusCard(t, searchTestRegistry(t), "Candy Grapple")
	e, cfg, reg := bargainEngine(t, spell)
	bears := seedBattlefield(t, e, reg, "Grizzly Bears")
	o := e.G.Obj(bears)
	if o == nil || o.IsToken || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears = %+v, want a nontoken battlefield permanent", o)
	}
	if o.Face() == nil || o.Face().IsArtifact() || o.Face().IsEnchantment() {
		t.Fatal("precondition: Grizzly Bears must be neither an artifact nor an enchantment")
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if id == bears {
			continue
		}
		if p := e.G.Obj(id); p != nil && p.Face() != nil {
			t.Fatalf("precondition: the bears must be alone on the battlefield, found %s", p.Face().Name)
		}
	}
	candy := searchMoveByName(t, e, "Candy Grapple", state.ZHand)
	if e.G.Obj(candy).Zone != state.ZHand || !e.HasKeyword(candy, "Bargain") {
		t.Fatal("precondition: Candy Grapple in hand with Bargain")
	}
	addMana(t, e, 0, "BB")
	if !bargainHasCastMode(e, candy, "") {
		t.Fatalf("plain cast option missing: %+v", e.Pending().Options)
	}
	if bargainHasCastMode(e, candy, "bargained") {
		t.Fatalf("bargained mode offered with only a nontoken creature: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)
}

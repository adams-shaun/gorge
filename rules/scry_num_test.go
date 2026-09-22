package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

const scryNumStaticSrc = "Name:Extra Scry\nManaCost:U\nTypes:Enchantment\n" +
	"S:Mode$ ScryNum | ValidPlayer$ You | Num$ 2 | Optional$ True\nOracle:x\n"
const scryOneForScryNumSrc = "Name:Scry One\nManaCost:U\nTypes:Sorcery\n" +
	"A:SP$ Scry | Defined$ You | ScryNum$ 1\nOracle:x\n"

func scryNumFixture(t *testing.T, seed uint64) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	stat := card(t, scryNumStaticSrc)
	scry := card(t, scryOneForScryNumSrc)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"}, Tokens: map[string]*cards.Card{},
		Decks: [][]*cards.Card{
			append([]*cards.Card{stat, scry}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	statID := moveByName(t, e, 0, "Extra Scry", state.ZBattlefield)
	scryID := moveByName(t, e, 0, "Scry One", state.ZHand)
	addMana(t, e, 0, "U")
	return e, cfg, scryID, statID
}

func TestScryNumStaticAddsTwoCards(t *testing.T) {
	e, cfg, id, statID := scryNumFixture(t, 701)
	if o := e.G.Obj(statID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: ScryNum static %v not on seat 0 battlefield: %+v", statID, e.G.Obj(statID))
	}
	if n := len(e.G.Zone(state.ZLibrary, 0)); n < 3 {
		t.Fatalf("precondition: library has %d cards, need at least 3", n)
	}
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected ScryNum may-look election, got %+v", d)
	}
	if d.Min != 0 || d.Max != 1 || len(d.Options) != 1 {
		t.Fatalf("election = %+v, want one optional static", d)
	}
	submitChoices(t, e, 0)
	d = e.Pending()
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected Scry arrange after accepting extra look, got %+v", d)
	}
	if d.Max != 3 || len(d.Options) != 3 {
		t.Fatalf("KArrange Max/options = %d/%d, want 3/3", d.Max, len(d.Options))
	}
	for i, o := range d.Options {
		if o.Kind != "bottom" {
			t.Fatalf("option %d Kind = %q, want bottom", i, o.Kind)
		}
	}
	submitChoices(t, e, 0)
	if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after arrange, pending = %+v: Scry must complete", d)
	}
	replayCheck(t, e, cfg)
}

func TestScryNumStaticIsRegistered(t *testing.T) {
	if !effects.Supported()["stat:ScryNum"] {
		t.Fatal("stat:ScryNum not reported by effects.Supported()")
	}
}

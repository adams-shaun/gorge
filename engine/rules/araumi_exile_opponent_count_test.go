package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// araumi_exile_opponent_count_test.go pins Araumi of the Dead Tide's cost
// "{T}, Exile cards from your graveyard equal to the number of opponents you
// have" (Cost$ T ExileFromGrave<X/Card>, the face's SVar X counting opponents).
// The SVar fixes the count: the activation must pose no X announcement and
// exile exactly as many cards as the controller has opponents
// (pay.FixLifeXCost folds the announced part to a fixed N).

func TestAraumiExilesOpponentCount(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	araumi := mustCorpusCard(t, reg, "Araumi of the Dead Tide")
	// Precondition: the corpus card really carries the shape under test.
	if got := araumi.Faces[0].SVars["X"]; got != "PlayerCountOpponents$Amount" {
		t.Fatalf("corpus Araumi SVar X = %q, want PlayerCountOpponents$Amount", got)
	}
	shaped := false
	for _, ab := range araumi.Faces[0].Abilities {
		if strings.Contains(ab.Params["Cost"], "ExileFromGrave<X/Card>") {
			shaped = true
		}
	}
	if !shaped {
		t.Fatal("corpus Araumi lost its ExileFromGrave<X/Card> cost -- the pin is vacuous")
	}
	bear := card(t, xMinParamBearSrc)
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b", "c"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{bear}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	araumiID := onBoardReadyCard(t, e, 0, araumi)
	bearID := findByName(e, "Bear", 0)
	if bearID == 0 {
		t.Fatal("fixture Bear not in seat 0's zones")
	}
	// Five graveyard cards: the Bear (the ability's target) and four Mountains.
	grave := []state.ObjID{bearID}
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if len(grave) == 5 {
			break
		}
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			grave = append(grave, id)
		}
	}
	for _, id := range grave {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZGraveyard})
	}
	e.pending = nil
	e.Advance()
	if n := len(e.G.Zone(state.ZGraveyard, 0)); n != 5 {
		t.Fatalf("setup: seat 0's graveyard holds %d cards, want 5", n)
	}
	if got := len(e.G.Players); got != 3 {
		t.Fatalf("setup: %d players, want 3 (two opponents)", got)
	}
	opt := abilityOption(t, e, araumiID, 0)
	submitChoices(t, e, opt.Index)
	exiledAsk := false
	for range 6 {
		d := e.Pending()
		if d == nil || d.Player != 0 || (d.Source != araumiID && !strings.HasPrefix(d.Prompt, "Choose a target for Araumi")) {
			break
		}
		if d.Prompt == "Choose a value for X" {
			t.Fatalf("Araumi posed an X announcement %+v; SVar X fixes the count at the opponent count", d)
		}
		var picks []int
		for _, o := range d.Options {
			switch {
			case o.Kind == "exilecost" && o.Obj != bearID && len(picks) < d.Max:
				picks = append(picks, o.Index)
			case o.Kind != "exilecost" && o.Obj == bearID:
				picks = append(picks, o.Index)
			}
		}
		if len(d.Options) > 0 && d.Options[0].Kind == "exilecost" {
			exiledAsk = true
			if d.Min != 2 || d.Max != 2 {
				t.Fatalf("exile-cost ask = Min %d Max %d, want exactly 2 (the opponent count)", d.Min, d.Max)
			}
		}
		if len(picks) == 0 {
			t.Fatalf("unexpected decision during activation: %+v", d)
		}
		submitChoices(t, e, picks...)
	}
	if !exiledAsk {
		t.Fatal("no exile-cost ask was posed: the activation never paid its exile cost")
	}
	exiled := 0
	for _, id := range grave {
		if e.G.Obj(id).Zone == state.ZExile {
			exiled++
		}
	}
	if exiled != 2 {
		t.Fatalf("exiled %d graveyard cards as the cost, want exactly 2 (the opponent count)", exiled)
	}
	if got := e.G.Obj(bearID).Zone; got != state.ZGraveyard {
		t.Fatalf("target Bear zone = %s, want still in the graveyard", got)
	}
}

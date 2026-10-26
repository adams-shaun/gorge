package rules

// CR 107.4f/119.4: a cost of two Phyrexian pips may mix faces. At 3 life the
// first pip can be paid with 2 life, leaving the second pip's colour face
// payable from an untapped source. The offer-side source probe must price the
// still-unannounced Phyrexian pip as a mana component, not treat it as "no
// mana payment" and withhold the life face.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const phyrexianTwoPipsSwamp = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"

func phyrexianTwoPipsPipAsk(t *testing.T, e *Engine, bearer state.ObjID) *decision.Decision {
	t.Helper()
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while waiting for a Phyrexian pip ask")
		}
		if d.Kind == decision.KTarget {
			idx := indexOfObjOption(d, bearer)
			if idx < 0 {
				t.Fatalf("equip target ask omits the bearer: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			continue
		}
		if d.Kind == decision.KChoose {
			for _, o := range d.Options {
				if o.Kind == "pay_B" || o.Kind == "pay_life" {
					return d
				}
			}
		}
		t.Fatalf("unexpected decision while waiting for a pip ask: %+v", d)
	}
	t.Fatal("no Phyrexian pip ask appeared")
	return nil
}

func phyrexianTwoPipsFaces(d *decision.Decision) (colour, life int) {
	colour, life = -1, -1
	for _, o := range d.Options {
		switch o.Kind {
		case "pay_B":
			colour = o.Index
		case "pay_life":
			life = o.Index
		}
	}
	return colour, life
}

func TestPhyrexianTwoPipsLifeThenColour(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	lashwrithe, ok := reg.Lookup("Lashwrithe")
	if !ok {
		t.Fatal("Lashwrithe missing from the corpus registry")
	}
	bears, ok := reg.Lookup("Grizzly Bears")
	if !ok {
		t.Fatal("Grizzly Bears missing from the corpus registry")
	}
	e := crAbortEngine(t, reg, "ur-delver")
	lash := onBoardCard(t, e, 0, lashwrithe)
	bearer := onBoardCard(t, e, 0, bears)
	var swamps []state.ObjID
	for i := 0; i < 3; i++ {
		swamps = append(swamps, onBoard(t, e, 0, phyrexianTwoPipsSwamp))
	}
	if d := 3 - e.G.Players[0].Life; d != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: d})
	}
	if got := e.G.Players[0].Life; got != 3 {
		t.Fatalf("precondition: life = %d, want 3", got)
	}
	if got := e.G.Obj(lash).Face().Abilities; len(got) == 0 {
		t.Fatal("precondition: Lashwrithe has no compiled abilities")
	}
	e.askPriority(0)

	opt := abilityOptionByLabel(t, e, lash, "Equip")
	submitChoices(t, e, opt.Index)

	first := phyrexianTwoPipsPipAsk(t, e, bearer)
	colour, life := phyrexianTwoPipsFaces(first)
	if colour < 0 || life < 0 {
		t.Fatalf("first pip at 3 life = %+v, want both pay_B and pay_life", first.Options)
	}
	submitChoices(t, e, life)

	second := phyrexianTwoPipsPipAsk(t, e, bearer)
	colour, life = phyrexianTwoPipsFaces(second)
	if colour < 0 {
		t.Fatalf("second pip after committing 2 life = %+v, want pay_B", second.Options)
	}
	if life >= 0 {
		t.Fatalf("second pip after committing 2 life = %+v, want no pay_life (1 life left)", second.Options)
	}
	submitChoices(t, e, colour)

	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision after the second pip")
		}
		if d.Kind == decision.KPriority {
			break
		}
		if d.Kind == decision.KTarget {
			idx := indexOfObjOption(d, bearer)
			if idx < 0 {
				t.Fatalf("equip target ask omits the bearer: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			continue
		}
		tapped := false
		for _, o := range d.Options {
			if o.Kind != "activate" || o.Obj == 0 {
				continue
			}
			if e.G.Obj(o.Obj).Face().Name != "Swamp" || e.G.Obj(o.Obj).Tapped {
				continue
			}
			submitChoices(t, e, o.Index)
			tapped = true
			break
		}
		if !tapped {
			t.Fatalf("payment window offers no untapped Swamp: %+v", d.Options)
		}
	}

	if got := e.G.Players[0].Life; got != 1 {
		t.Fatalf("life after the activation = %d, want 1 (2 life for the first pip, {B} for the second)", got)
	}
	tapped := 0
	for _, id := range swamps {
		if e.G.Obj(id).Tapped {
			tapped++
		}
	}
	if tapped != 1 {
		t.Fatalf("%d Swamps tapped, want exactly 1 (the second pip's colour face)", tapped)
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the equip activation never reached the stack")
	}
}

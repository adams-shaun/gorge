package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// allSunsDawnText is All Suns' Dawn's printed expansion: five per-colour
// ChangeZone links, each LockInText$ True, then the self-exile. The Oracle text
// deliberately prints no colour word, which is why the substitution must not
// reach the script's filters (CR 612.2).
const allSunsDawnText = "Name:All Suns' Dawn\n" +
	"ManaCost:4 G\n" +
	"Types:Sorcery\n" +
	"A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.White+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ ReturnU\n" +
	"SVar:ReturnU:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.Blue+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ ReturnB\n" +
	"SVar:ReturnB:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.Black+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ ReturnR\n" +
	"SVar:ReturnR:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.Red+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ ReturnG\n" +
	"SVar:ReturnG:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.Green+YouOwn | TargetMin$ 0 | TargetMax$ 1 | LockInText$ True | SubAbility$ DBExile\n" +
	"SVar:DBExile:DB$ ChangeZone | Origin$ Stack | Destination$ Exile\n" +
	"Oracle:For each color, return up to one target card of that color from your graveyard to your hand. Exile All Suns' Dawn.\n"

// TestChangeTextAllSunsDawnStillOneOfEachColor drives the engine with All
// Suns' Dawn's text already rewritten White -> Blue (the state a text-changing
// effect produces). The locked per-colour chain must still offer one target of
// each colour, so a white and a blue card in the graveyard both return.
//
// The rewrite is applied to the face directly rather than by casting Mind
// Bend: All Suns' Dawn is a sorcery, and Mind Bend's `ValidTgts$ Permanent`
// cannot target it in any zone (and a real text change ends at the zone change
// that brings the card to the stack, CR 400.7). WithTextChanges is exactly the
// rewrite Mind Bend performs on a legal target, so this exercises the same
// face the cast census reads.
func TestChangeTextAllSunsDawnStillOneOfEachColor(t *testing.T) {
	t.Parallel()
	allSunsDawn, diags := cards.ParseBytes("all_suns_dawn.txt", []byte(allSunsDawnText))
	if len(diags) != 0 {
		t.Fatalf("parse diagnostics: %v", diags)
	}
	if d := allSunsDawn.Link(); len(d) != 0 {
		t.Fatalf("link diagnostics: %v", d)
	}
	allSunsDawn.Faces[0] = allSunsDawn.Faces[0].WithTextChanges([]cards.TextSub{{From: "White", To: "Blue"}})

	white := card(t, "Name:Test White Card\nManaCost:W\nTypes:Creature Human\nPT:1/1\nOracle:x\n")
	blue := card(t, "Name:Test Blue Card\nManaCost:U\nTypes:Creature Merfolk\nPT:1/1\nOracle:x\n")
	e, cfg, _ := corpusDeckEngine(t, []*cards.Card{allSunsDawn}, []*cards.Card{white, blue})

	var whiteID, blueID state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner != 0 {
			continue
		}
		switch {
		case o.Card == white && o.Zone == state.ZBattlefield:
			whiteID = o.ID
		case o.Card == blue && o.Zone == state.ZBattlefield:
			blueID = o.ID
		}
	}
	if whiteID == 0 || blueID == 0 {
		t.Fatalf("setup: white=%d blue=%d, both must be on the battlefield", whiteID, blueID)
	}
	for _, id := range []state.ObjID{whiteID, blueID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	}
	if e.G.Obj(whiteID).Zone != state.ZGraveyard || e.G.Obj(blueID).Zone != state.ZGraveyard {
		t.Fatalf("setup: the two colour cards must be in the graveyard")
	}

	addMana(t, e, 0, "GGGGG")
	castCardNow(t, e, "All Suns' Dawn")

	sawAsk := false
	whiteOffered, blueOffered := false, false
	for i := 0; i < 80; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if sawAsk && len(e.G.Stack) == 0 {
			break
		}
		switch d.Kind {
		case decision.KTarget:
			sawAsk = true
			switch {
			case indexOfObjOption(d, whiteID) >= 0:
				whiteOffered = true
				submitChoices(t, e, indexOfObjOption(d, whiteID))
			case indexOfObjOption(d, blueID) >= 0:
				blueOffered = true
				submitChoices(t, e, indexOfObjOption(d, blueID))
			default:
				submitChoices(t, e) // TargetMin$ 0 for a colour with no card
			}
		case decision.KPriority:
			passOnceP(t, e)
		default:
			t.Fatalf("unexpected pending decision %v (%s)", d.Kind, d.Prompt)
		}
	}
	if !whiteOffered || !blueOffered {
		t.Fatalf("locked chain offered white=%v blue=%v; the White target step must still offer the white card", whiteOffered, blueOffered)
	}
	if got := e.G.Obj(whiteID).Zone; got != state.ZHand {
		t.Errorf("white card zone = %v, want hand (the White chain step must still return it)", got)
	}
	if got := e.G.Obj(blueID).Zone; got != state.ZHand {
		t.Errorf("blue card zone = %v, want hand", got)
	}
	replayCheck(t, e, cfg)
}

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// tokenETBProbeName is the corpus's plain "Whenever another creature you
// control enters, you gain 1 life" carrier (Virulent Emissary). The token
// mint in this test is a creature, so one non-echo resolution shows up as
// exactly +1 life.
const tokenETBProbeName = "Virulent Emissary"

// tokenETBEngine seats an optional Panharmonicon carrier, the life-gain probe
// and an artifact token forge on seat 0's battlefield. The forge's
// AB$ Token ability is the real mint path (a spell/ability creating a
// creature token), not a hand-built TokenCreate event.
func tokenETBEngine(t *testing.T, carrierName string) (*Engine, state.ObjID) {
	t.Helper()
	var carrier *cards.Card
	deck := []*cards.Card{}
	if carrierName != "" {
		carrier = corpusCard(t, carrierName)
		deck = append(deck, carrier)
	}
	probe := corpusCard(t, tokenETBProbeName)
	maker := cardByName(t, tokenForgeSrc("g_1_1_squirrel"))
	deck = append(deck, probe, maker)
	e, _ := tokenReplGame(t, 71, deck...)

	var carrierID state.ObjID
	if carrier != nil {
		carrierID = moveSeededCard(t, e, 0, carrier, state.ZBattlefield)
		o := e.G.Obj(carrierID)
		found := false
		for _, st := range o.Face().Statics {
			if st.Mode == "Panharmonicon" {
				found = true
			}
		}
		if o.Zone != state.ZBattlefield || o.Controller != 0 || !found {
			t.Fatalf("precondition: carrier %d is not a Panharmonicon static on seat 0's battlefield: %+v", carrierID, o)
		}
	}
	probeID := moveSeededCard(t, e, 0, probe, state.ZBattlefield)
	po := e.G.Obj(probeID)
	if po.Zone != state.ZBattlefield || po.Controller != 0 ||
		len(po.Face().Triggers) == 0 || po.Face().Triggers[0].Mode != "ChangesZone" {
		t.Fatalf("precondition: life-gain probe %d is not a ChangesZone trigger on seat 0's battlefield: %+v", probeID, po)
	}
	makerID := moveSeededCard(t, e, 0, maker, state.ZBattlefield)
	if o := e.G.Obj(makerID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: token forge %d is not on seat 0's battlefield: %+v", makerID, o)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: seat 0 life before the mint = %d, want 20", got)
	}
	return e, makerID
}

// TestPanharmoniconDoublesTokenETB pins the cr-g253 defect: a Panharmonicon
// static whose ValidMode$ names ChangesZone/ChangesZoneAll must double an ETB
// trigger caused by a TOKEN entering. A token is minted by a TokenCreate
// event (no MoveZone), so before the fix panharmoniconModes returned nil, the
// Destination$ gate read the event's zero To (Library) and ValidCause$ had no
// cause, and the echo never fired.
//
// The no-carrier control proves the mint fires exactly one non-echo trigger
// (life +1); with the carrier the same mint must resolve twice (life +2).
func TestPanharmoniconDoublesTokenETB(t *testing.T) {
	t.Parallel()

	t.Run("no carrier fires exactly one trigger", func(t *testing.T) {
		t.Parallel()
		e, maker := tokenETBEngine(t, "")
		activateTokenForge(t, e, maker)
		drainTriggers(t, e)
		if got := countTokensNamedOnSeat(t, e, 0, "Squirrel Token"); got != 1 {
			t.Fatalf("precondition: minted %d Squirrel Tokens, want 1", got)
		}
		if got := e.G.Players[0].Life; got != 21 {
			t.Fatalf("control: one ETB trigger gained to life %d, want 21", got)
		}
	})

	for _, carrier := range []string{"Starfield Vocalist", "Panharmonicon"} {
		t.Run(carrier, func(t *testing.T) {
			t.Parallel()
			e, maker := tokenETBEngine(t, carrier)
			activateTokenForge(t, e, maker)
			drainTriggers(t, e)
			if got := countTokensNamedOnSeat(t, e, 0, "Squirrel Token"); got != 1 {
				t.Fatalf("precondition: minted %d Squirrel Tokens, want 1", got)
			}
			if got := e.G.Players[0].Life; got != 22 {
				t.Fatalf("%s doubled the token's ETB trigger to life %d, want 22 (the token entering is a ChangesZone cause)", carrier, got)
			}
		})
	}
}

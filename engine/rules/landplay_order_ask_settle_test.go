package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A land play parked on an order ask (CR 616.1) is over once that ask is
// answered, whatever the chosen replacement does with the entry. A play left
// in flight would skip the next play's count (CR 305.2b) and let a later
// non-play entry of the same card read as a land play (CR 305.1, 305.4).
const (
	orderSettleBounceLandSrc = "Name:Bounceland\nTypes:Land\n" +
		"R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplaceWith$ ToYard | Description$ x\n" +
		"SVar:ToYard:DB$ ChangeZone | Origin$ All | Destination$ Graveyard | Defined$ ReplacedCard\nOracle:x\n"
	orderSettleMayBounceLandSrc = "Name:Mayland\nTypes:Land\n" +
		"R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | Optional$ True | ReplaceWith$ ToYard | Description$ x\n" +
		"SVar:ToYard:DB$ ChangeZone | Origin$ All | Destination$ Graveyard | Defined$ ReplacedCard\nOracle:x\n"
	orderSettleTapperSrc = "Name:Tapper\nManaCost:2 W\nTypes:Enchantment\n" +
		"R:Event$ Moved | Destination$ Battlefield | ValidCard$ Land | ActiveZones$ Battlefield | ReplacementResult$ Updated | ReplaceWith$ TapOnEntry | Description$ x\n" +
		"SVar:TapOnEntry:DB$ Tap | Defined$ ReplacedCard | ETB$ True\nOracle:x\n"
	orderSettlePlainLandSrc = "Name:Plainland\nTypes:Land\nOracle:x\n"
	orderSettleAskLandSrc   = "Name:Askland\nTypes:Land\n" +
		"R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplaceWith$ SacFirst | Description$ x\n" +
		"SVar:SacFirst:DB$ Sacrifice | Defined$ You | SacValid$ Land | SubAbility$ ThenYard\n" +
		"SVar:ThenYard:DB$ ChangeZone | Origin$ All | Destination$ Graveyard | Defined$ ReplacedCard\nOracle:x\n"

	orderSettleChargeLandSrc = "Name:Chargeland\nTypes:Land\nK:etbCounter:CHARGE:2\nOracle:x\n"
	orderSettleTwiceSrc      = "Name:Twice\nManaCost:2 G\nTypes:Enchantment\n" +
		"R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Land.YouCtrl | ValidCounterType$ CHARGE | ReplaceWith$ TwiceBody | Description$ x\n" +
		"SVar:TwiceBody:DB$ ReplaceCounter | ValidCounterType$ CHARGE | Amount$ N\n" +
		"SVar:N:ReplaceCount$CounterNum/Twice\nOracle:x\n"
	orderSettlePlusSrc = "Name:Plusone\nManaCost:G\nTypes:Enchantment\n" +
		"R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Land.YouCtrl | ValidCounterType$ CHARGE | ReplaceWith$ PlusBody | Description$ x\n" +
		"SVar:PlusBody:DB$ ReplaceCounter | ValidCounterType$ CHARGE | Amount$ M\n" +
		"SVar:M:ReplaceCount$CounterNum/Plus.1\nOracle:x\n"
)

// orderSettlePlay submits the play_land option for land.
func orderSettlePlay(t *testing.T, e *Engine, land state.ObjID) {
	t.Helper()
	e.pending = nil
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want priority", d)
	}
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == land {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatalf("submit play_land: %v", err)
			}
			return
		}
	}
	t.Fatalf("land %d not offered: %+v", land, d.Options)
}

// orderSettleAnswer answers the pending order ask with the option whose source
// is from, after checking the ask is the one the test expects.
func orderSettleAnswer(t *testing.T, e *Engine, from state.ObjID, kind string) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KReplacement {
		t.Fatalf("pending = %+v, want a replacement ask", d)
	}
	for _, o := range d.Options {
		if o.Obj == from && o.Kind == kind {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatalf("answer: %v", err)
			}
			return
		}
	}
	t.Fatalf("no %q option from %d in %+v", kind, from, d.Options)
}

func orderSettleLogged(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.LandPlayed {
			n++
		}
	}
	return n
}

// The chosen replacement discards the entry, either as the picked side of a
// competition or as an accepted lone "you may": the play is counted once, is
// disarmed, and the next play counts as usual.
func TestLandPlayOrderAskReplacedEntryDisarms(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, land, kind string
		tapper, bodyAsks bool
	}{
		{"competition", orderSettleBounceLandSrc, "replacement", true, false},
		{"competition with an asking body", orderSettleAskLandSrc, "replacement", true, true},
		{"optional", orderSettleMayBounceLandSrc, "apply", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := mayPlayBase(t)
			if tc.bodyAsks {
				onBoardGrant(t, e, 0, orderSettlePlainLandSrc)
				onBoardGrant(t, e, 0, orderSettlePlainLandSrc)
			}
			if tc.tapper {
				onBoardGrant(t, e, 0, orderSettleTapperSrc)
			}
			playWatch := onBoardGrant(t, e, 0, entryOrderLandPlaySrc)
			land := handCard(e, card(t, tc.land), 0)
			orderSettlePlay(t, e, land)
			if z := e.G.Obj(land).Zone; z != state.ZHand || !e.etbLandPlay {
				t.Fatalf("precondition: zone=%v armed=%v, want a play parked in hand", z, e.etbLandPlay)
			}
			orderSettleAnswer(t, e, land, tc.kind)
			if tc.bodyAsks {
				d := e.Pending()
				if d == nil || d.Kind == decision.KPriority || d.Kind == decision.KReplacement {
					t.Fatalf("pending = %+v, want the replacement body's own ask", d)
				}
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
					t.Fatalf("answer the body's ask: %v", err)
				}
			}
			if z := e.G.Obj(land).Zone; z != state.ZGraveyard {
				t.Fatalf("land zone = %v, want graveyard (the replacement applied)", z)
			}
			if e.etbLandPlay || e.etbLandLogged || e.etbLandObj != 0 {
				t.Fatalf("play still armed: play=%v logged=%v obj=%d", e.etbLandPlay, e.etbLandLogged, e.etbLandObj)
			}
			if got, n := e.G.Players[0].LandsPlayed, orderSettleLogged(e); got != 1 || n != 1 {
				t.Fatalf("LandsPlayed=%d LandPlayed events=%d, want 1 and 1 (CR 305.1)", got, n)
			}
			// A put onto the battlefield is not a land play (CR 305.4). The
			// land's own replacement is stepped around with a raw log entry
			// followed by the trigger walk.
			move := events.Event{Kind: events.MoveZone, Obj: land, From: state.ZGraveyard, To: state.ZBattlefield}
			stored := events.Emit(e.G, e.L, move)
			e.refreshEntryTriggerTables()
			e.checkTriggers(&stored, nil, 0, 0, false)
			if z := e.G.Obj(land).Zone; z != state.ZBattlefield {
				t.Fatalf("precondition: land zone = %v after the put", z)
			}
			if n := observedTriggerCount(e, playWatch); n != 0 {
				t.Fatalf("land-play triggers after a non-play entry = %d, want 0", n)
			}
			// The next land play counts (CR 305.2b).
			e.G.Players[0].LandsPlayed = 0
			next := handCard(e, card(t, orderSettlePlainLandSrc), 0)
			orderSettlePlay(t, e, next)
			answerReplacementOrderAsk(t, e)
			if z := e.G.Obj(next).Zone; z != state.ZBattlefield {
				t.Fatalf("next land zone = %v, want battlefield", z)
			}
			if got, n := e.G.Players[0].LandsPlayed, orderSettleLogged(e); got != 1 || n != 2 {
				t.Fatalf("next play: LandsPlayed=%d LandPlayed events=%d, want 1 and 2", got, n)
			}
		})
	}
}

// An accepted "no" on the lone "you may" lets the land enter: still one play,
// with its land-play trigger.
func TestLandPlayOrderAskDeclinedOptionalKeepsTrigger(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	playWatch := onBoardGrant(t, e, 0, entryOrderLandPlaySrc)
	land := handCard(e, card(t, orderSettleMayBounceLandSrc), 0)
	orderSettlePlay(t, e, land)
	if z := e.G.Obj(land).Zone; z != state.ZHand {
		t.Fatalf("precondition: zone=%v, want the play parked in hand", z)
	}
	orderSettleAnswer(t, e, land, "decline")
	if z := e.G.Obj(land).Zone; z != state.ZBattlefield {
		t.Fatalf("land zone = %v, want battlefield", z)
	}
	if got, n := e.G.Players[0].LandsPlayed, orderSettleLogged(e); got != 1 || n != 1 || e.etbLandPlay {
		t.Fatalf("LandsPlayed=%d events=%d armed=%v, want 1, 1, false", got, n, e.etbLandPlay)
	}
	if n := observedTriggerCount(e, playWatch); n != 1 {
		t.Fatalf("land-play triggers = %d, want 1 (CR 305.1)", n)
	}
}

// A land entering with counters under two non-commuting counter replacements
// parks on the entry-count order ask: the play stays in flight until the
// answered entry lands.
func TestLandPlayEntryCounterOrderKeepsTrigger(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	onBoardGrant(t, e, 0, orderSettleTwiceSrc)
	onBoardGrant(t, e, 0, orderSettlePlusSrc)
	playWatch := onBoardGrant(t, e, 0, entryOrderLandPlaySrc)
	land := handCard(e, card(t, orderSettleChargeLandSrc), 0)
	orderSettlePlay(t, e, land)
	if d := e.Pending(); d == nil || d.Kind != decision.KReplacement {
		t.Fatalf("pending = %+v, want the entry-count order ask", d)
	}
	if len(e.replChoices) == 0 || e.replChoices[0].kind != replChoiceEntryOrder {
		t.Fatalf("precondition: parked choices = %+v, want an entry-order park", e.replChoices)
	}
	if z := e.G.Obj(land).Zone; z != state.ZHand {
		t.Fatalf("precondition: zone=%v, want the play parked in hand", z)
	}
	answerReplacementOrderAsk(t, e)
	if z := e.G.Obj(land).Zone; z != state.ZBattlefield {
		t.Fatalf("land zone = %v, want battlefield", z)
	}
	if got, n := e.G.Players[0].LandsPlayed, orderSettleLogged(e); got != 1 || n != 1 || e.etbLandPlay || e.etbLandLogged {
		t.Fatalf("LandsPlayed=%d events=%d armed=%v logged=%v, want 1, 1, false, false", got, n, e.etbLandPlay, e.etbLandLogged)
	}
	if n := observedTriggerCount(e, playWatch); n != 1 {
		t.Fatalf("land-play triggers = %d, want 1 (CR 305.1)", n)
	}
}

package rules

import (
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The ValidTgts$ player qualifier is enforced at both target sites (the offer
// in candidatesFor and the resolution recheck in legalTargets), through the
// one shared grammar effects.MatchesPlayerSpecFrom. These tests pin, per
// qualifier shape, that the asker's own seat is not offered for an Opponent
// spec and no opponent is offered for a You spec, that a mixed object,player
// spec keeps the object half, and that the recheck rejects a bypassed choice
// the same way.

// etbGainControlCard builds a one-trigger creature whose ETB poses a player
// target ask governed by ValidTgts$ spec (the Sleeper Agent shape).
func etbGainControlCard(t testing.TB, name, spec string) *cards.Card {
	t.Helper()
	src := "Name:" + name + "\nTypes:Creature\nPT:2/2\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCtl\n" +
		"SVar:TrigCtl:DB$ GainControl | Defined$ Self | ValidTgts$ " + spec + "\nOracle:x\n"
	return card(t, src)
}

// etbTriggerEngine builds a seat-0-start engine, adds extras (first) and the
// trigger creature (last) to seat 0's library, moves every one of them onto
// the battlefield before the engine's first Advance so their ETB triggers
// fire with the opening turn, and returns the engine. The trigger card is
// the LAST object added.
func etbTriggerEngine(t *testing.T, n int, extras []*cards.Card, trig *cards.Card) (*Engine, *state.Object) {
	t.Helper()
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	for i := range names {
		names[i] = string(rune('a' + i))
		decks[i] = mountainDeck(t, 40)
	}
	// The ETB trigger card rides seat 0's deck so its Zones bookkeeping is
	// ordinary; the pre-Advance MoveZone is what fires it.
	decks[0] = append(append(extras, trig), decks[0]...)
	e := New(seatZeroStart(Config{Seed: 721, Names: names, Decks: decks}))
	var trigObj *state.Object
	for _, c := range append(extras, trig) {
		obj := e.G.AddObject(c, 0)
		if c == trig {
			trigObj = obj
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: obj.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	e.Advance()
	return e, trigObj
}

// playerTargetOptions returns the pending decision's player options (seats).
func playerTargetOptions(t *testing.T, d *decision.Decision) []state.PlayerID {
	t.Helper()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("wanted a KTarget ask, got %+v", d)
	}
	var out []state.PlayerID
	for _, o := range d.Options {
		if o.Kind == "player" {
			out = append(out, o.Player)
		}
	}
	return out
}

// TestSleeperAgentTargetAskExcludesTheController drives Sleeper Agent's real
// corpus ETB (DB$ GainControl | ValidTgts$ Opponent) end to end: the ask must
// offer the opponent and NOT the spell's own controller. The pre-fix engine
// offered every living seat (the controller was option 0) and the recheck
// accepted any of them, so the Opponent qualifier was never enforced.
func TestSleeperAgentTargetAskExcludesTheController(t *testing.T) {
	e, sleeper := etbTriggerEngine(t, 2, nil, choiceCorpusCard(t, "Sleeper Agent"))
	d := e.Pending()
	if got := playerTargetOptions(t, d); len(got) != 1 || got[0] != 1 {
		t.Fatalf("Opponent ask offered players %v, want exactly [1] (controller 0 excluded)", got)
	}
	if indexOfPlayerOption(d, 0) >= 0 {
		t.Fatalf("controller seat 0 is offered: %+v", d.Options)
	}
	// Offer and recheck agree: answering the offered seat 1 keeps the ask
	// answerable and the control transfer still happens (the downstream half
	// is pinned by TestSleeperAgentETBControlGoesToChosenOpponent).
	submitChoices(t, e, indexOfPlayerOption(d, 1))
	for e.Pending() != nil && e.Pending().Kind == decision.KPriority {
		submitChoicePass(t, e)
	}
	if e.G.Obj(sleeper.ID).Controller != 1 {
		t.Fatalf("Sleeper controller = %d, want 1", e.G.Obj(sleeper.ID).Controller)
	}
}

// TestSleeperAgentTargetAskOffersOnlyOpponentsAtThreeSeats: with three seats
// the Opponent ask offers exactly the two non-controllers, never seat 0.
func TestSleeperAgentTargetAskOffersOnlyOpponentsAtThreeSeats(t *testing.T) {
	e, _ := etbTriggerEngine(t, 3, nil, choiceCorpusCard(t, "Sleeper Agent"))
	d := e.Pending()
	got := playerTargetOptions(t, d)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("3-seat Opponent ask offered players %v, want exactly [1 2]", got)
	}
}

// TestPlayerTargetAskYouSpecOffersOnlyTheController: ValidTgts$ You (the one
// raw corpus line's shape) offers only the asker, never an opponent.
func TestPlayerTargetAskYouSpecOffersOnlyTheController(t *testing.T) {
	e, you := etbTriggerEngine(t, 2, nil, etbGainControlCard(t, "YouAgent", "You"))
	d := e.Pending()
	if got := playerTargetOptions(t, d); len(got) != 1 || got[0] != 0 {
		t.Fatalf("You ask offered players %v, want exactly [0]", got)
	}
	// Answer the (only) legal option: the recheck accepts it and resolution
	// completes -- controller unchanged (0 gains control of its own creature).
	idx := indexOfPlayerOption(d, 0)
	if idx < 0 {
		t.Fatalf("no option for the controller: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	if e.G.Obj(you.ID).Controller != 0 {
		t.Fatalf("controller = %d, want 0", e.G.Obj(you.ID).Controller)
	}
}

// TestPlayerTargetAskMixedSpecOffersCreatureAndOpponentNotTheController: the
// comma-split must preserve the object half (Creature) while the player half
// (Opponent) excludes the asker.
func TestPlayerTargetAskMixedSpecOffersCreatureAndOpponentNotTheController(t *testing.T) {
	bear := card(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e, mix := etbTriggerEngine(t, 2, []*cards.Card{bear}, etbGainControlCard(t, "MixedAgent", "Creature,Opponent"))
	var bearID state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Bear" {
			bearID = id
		}
	}
	if bearID == 0 || mix == nil {
		t.Fatal("fixture objects missing")
	}
	d := e.Pending()
	sawOpponent, sawCreature, sawSelf := false, false, false
	for _, o := range d.Options {
		switch {
		case o.Kind == "player" && o.Player == 1:
			sawOpponent = true
		case o.Kind == "player" && o.Player == 0:
			sawSelf = true
		case o.Kind != "player" && o.Obj == bearID:
			sawCreature = true
		}
	}
	if !sawOpponent || !sawCreature {
		t.Fatalf("mixed ask lost a half: opponent=%v creature=%v options=%+v", sawOpponent, sawCreature, d.Options)
	}
	if sawSelf {
		t.Fatalf("mixed ask offered the controller seat 0: %+v", d.Options)
	}
}

// TestLegalTargetsRecheckAppliesThePlayerSpec: a player target chosen through
// a path that bypassed the offer is judged by the same filter the offer uses,
// so the two sites cannot disagree. An Opponent spec drops the asker's own
// seat, a You spec drops every other seat, and the recognized qualified forms
// keep working.
func TestLegalTargetsRecheckAppliesThePlayerSpec(t *testing.T) {
	e := newSeats(t, 2)
	both := []state.Target{{IsPlayer: true, Player: 0}, {IsPlayer: true, Player: 1}}
	zones := []state.Zone{state.ZBattlefield}
	cases := []struct {
		spec string
		want []state.PlayerID
	}{
		{"Opponent", []state.PlayerID{1}},
		{"You", []state.PlayerID{0}},
		{"Any", []state.PlayerID{0, 1}},
		{"Player", []state.PlayerID{0, 1}},
		{"Player.Opponent", []state.PlayerID{1}},
		{"Creature,Opponent", []state.PlayerID{1}}, // the object half is judged by the object arm
	}
	for _, tc := range cases {
		legal := e.legalTargets(both, tc.spec, zones, 0, 0, 0)
		var got []state.PlayerID
		for _, t := range legal {
			if t.IsPlayer {
				got = append(got, t.Player)
			}
		}
		if len(got) != len(tc.want) || (len(got) > 0 && (got[0] != tc.want[0] || got[len(got)-1] != tc.want[len(tc.want)-1])) {
			t.Fatalf("spec %q recheck kept players %v, want %v", tc.spec, got, tc.want)
		}
	}
}

// The corpus census over distinct pure-player ValidTgts$ values (every
// comma-alternative's base is Player/Any/Opponent/You), classified
// semantically: a value "offers a seat" when MatchesPlayerSpecFrom matches at
// least one seat of a plain 2-seat game, else it fails closed (the AGENTS.md
// MatchesPlayerSpec convention). The two sets are pinned so a corpus update
// that adds an unhandled qualifier shape fails loudly here.

var censusTest struct {
	sync.Mutex
	reg *cards.Registry
}

// TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets is the census.
func TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets(t *testing.T) {
	censusTest.Lock()
	defer censusTest.Unlock()
	if censusTest.reg == nil {
		censusTest.reg = testutil.CorpusRegistry(t)
	}
	reg := censusTest.reg
	seen := map[string]bool{}
	for _, c := range reg.Cards {
		for _, face := range c.Faces {
			var sas []*cards.SA
			sas = append(sas, face.Abilities...)
			for _, tr := range face.Triggers {
				sas = append(sas, tr.Effect)
			}
			for _, r := range face.Repls {
				sas = append(sas, r.With)
			}
			for name := range face.SVars {
				if body := cards.ResolveSVar(face.SVars, name); body != nil {
					sas = append(sas, body)
				}
			}
			for _, sa := range sas {
				if sa == nil {
					continue
				}
				spec := sa.Params["ValidTgts"]
				if !purePlayerSpec(spec) {
					continue
				}
				seen[spec] = true
			}
		}
	}
	e := newSeats(t, 2)
	var offered, failClosed []string
	for spec := range seen {
		matched := false
		for p := state.PlayerID(0); int(p) < len(e.G.Players); p++ {
			if effects.MatchesPlayerSpecFrom(e.G, spec, p, 0, 0) {
				matched = true
			}
		}
		if matched {
			offered = append(offered, spec)
		} else {
			failClosed = append(failClosed, spec)
		}
	}
	sort.Strings(offered)
	sort.Strings(failClosed)
	// The recognized mob: bare Player/Any/Opponent/You and the two qualified
	// forms MatchesPlayerSpecFrom evaluates. Any.NotDefinedParentTarget,Player
	// offers seats through its bare Player alternative (the unhandled
	// NotDefinedParentTarget clause contributes nothing), so it is classified
	// by behaviour, not by its first alternative.
	wantOffered := []string{"Any", "Any.NotDefinedParentTarget,Player", "Opponent", "Player", "Player.Opponent", "Player.Other", "You"}
	wantFailClosed := []string{
		"Any.!Dinosaur", "Any.!Dragon", "Any.!IsCommander",
		"Opponent.wasDealtDamageThisGameBy Self",
		"Player.!CardOwner", "Player.!EnchantedBy", "Player.!TriggeredActivator",
		"Player.!TriggeredCardController", "Player.LostLifeThisTurn",
		"Player.Opponent+Active",
		"Player.OpponentToActive+hasFewerCreaturesInYardThanActive",
		"Player.OpponentToActive+hasMoreCardsInHandThanActive",
		"Player.OpponentToActive+hasMoreLifeThanActive",
		"Player.OpponentToActive+withMoreCreaturesThanActive",
		"Player.OpponentToActive+withMoreLandsThanActive",
		"Player.attackedWithCreaturesThisTurn",
		"Player.wasDealtCombatDamageThisTurnBySource",
		"Player.wasDealtDamageThisTurnBySource",
	}
	if len(seen) != 25 {
		t.Fatalf("census population moved: %d distinct pure-player values (was 25)", len(seen))
	}
	if !equalStrings(offered, wantOffered) {
		t.Fatalf("seat-offering values moved:\n got %v\nwant %v", offered, wantOffered)
	}
	if !equalStrings(failClosed, wantFailClosed) {
		t.Fatalf("fail-closed values moved:\n got %v\nwant %v", failClosed, wantFailClosed)
	}
}

// purePlayerSpec reports whether every comma-alternative of a ValidTgts$
// spec has a player base (the spec can only ever name players).
func purePlayerSpec(spec string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return false
	}
	for _, alt := range strings.Split(spec, ",") {
		base, _, _ := strings.Cut(strings.TrimSpace(alt), ".")
		switch base {
		case "Player", "Any", "Opponent", "You":
		default:
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

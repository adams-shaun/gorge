package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Task replplay1: a DB$ Play SA's ReplaceGraveyard$ Exile rider ("If that
// spell would be put into your graveyard this turn, exile it instead" —
// Goblin Dark-Dwellers' ETB) was never read: the played spell resolved and
// rested in the graveyard. The fix stamps the played spell's pay-time
// CastInfo with the new state.FlagReplaceGraveyard (provenance of the Play
// SA, plumbed from rules/resolution.go's "play" arm through beginPlay and
// pendingCast), and spellRestZone/spellFizzleZone read the bit to send the
// card to exile.
//
// Prevalence at the corpus pin: all 37 raw `ReplaceGraveyard$` occurrences
// are `Exile`; 34 files carry one on a Play-API line (the scope here), 2 on
// a static `S:` MayPlay$ grant (Kess, Dissident Mage; Maestros Ascendancy —
// out of scope, see the report), and the conditional sibling
// ReplaceGraveyardValid$ appears on exactly 2 of the 34 (Bilbo, Thief in the
// Night; Scholar of the Lost Trove) — left fail-closed: no flag, graveyard
// resting place.
//
// Goblin Dark-Dwellers and Lightning Bolt are in NO repo deck and NO legacy
// golden deck (measured: grepping the 34 Play-API carrier names against
// internal/testutil/decks/*.json returns nothing), so TestHeads is safe by
// construction; verified unmoved below anyway.

// gddEngine deals seat 0 a deck whose only non-mountain cards are a Goblin
// Dark-Dwellers and a Lightning Bolt, seat 1 all mountains.
func gddEngine(t *testing.T, reg *cards.Registry) (*Engine, Config) {
	t.Helper()
	gdd := searchCorpusCard(t, reg, "Goblin Dark-Dwellers")
	bolt := searchCorpusCard(t, reg, "Lightning Bolt")
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck0 := []*cards.Card{gdd, bolt}
	for len(deck0) < 40 {
		deck0 = append(deck0, mountain)
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 7402, Names: []string{"dwellers", "opponent"},
		Decks: [][]*cards.Card{deck0, deck1}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// gddDrive walks the whole Goblin Dark-Dwellers scenario: the ETB trigger's
// target ask (the graveyard's Bolt is the only option), the may-cast
// KModes/"play" ask answered "yes" (option 0), the played Bolt's own damage
// target, and the priority passes until everything settles. The walk stops
// once the Bolt has reached its resting zone (the scenario's own end) at a
// turn boundary — never mid-ask — rather than playing on into the
// opponent's next turn.
func gddDrive(t *testing.T, e *Engine, boltID state.ObjID, final state.Zone) {
	t.Helper()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			return
		}
		if o := e.G.Obj(boltID); o != nil && o.Zone == final &&
			(d.Kind == decision.KPriority || d.ResumeKind == "") && d.Kind != decision.KModes {
			return
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KTarget:
			submitChoices(t, e, 0)
		case decision.KModes:
			if d.ResumeKind == "play" {
				submitChoices(t, e, 0)
			} else {
				submitChoices(t, e)
			}
		default:
			t.Fatalf("unexpected decision %d: kind=%s resume=%q", i, d.Kind, d.ResumeKind)
		}
	}
	t.Fatal("the Goblin Dark-Dwellers drive did not settle within 40 decisions")
}

// TestGoblinDarkDwellersExilesThePlayedCard: the ETB trigger targets the
// graveyard's Lightning Bolt, the may-cast is taken, the free cast resolves
// — and the Bolt must rest in EXILE (the rider), not the graveyard. Before
// the fix the played card landed in the graveyard, because a free Play cast
// carried no CastFlags bit at all and spellRestZone fell to its
// ZGraveyard default.
func TestGoblinDarkDwellersExilesThePlayedCard(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := gddEngine(t, reg)
	boltID := searchMoveByName(t, e, "Lightning Bolt", state.ZGraveyard)
	searchMoveByName(t, e, "Goblin Dark-Dwellers", state.ZBattlefield)
	gddDrive(t, e, boltID, state.ZExile)

	if o := e.G.Obj(boltID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the played Bolt rests in %v, want exile (the ReplaceGraveyard$ Exile rider)", o)
	}
	// The provenance rides the pay-time CastInfo (the exact event a replay
	// marks the spell with): one cast_info carrying the new flag, nothing
	// else — a free Play cast emits no other CastInfo.
	infos := castInfosFor(e, boltID)
	if len(infos) != 1 {
		t.Fatalf("%d cast_info events for the played Bolt, want exactly 1", len(infos))
	}
	if infos[0][0] != "replacegraveyard" {
		t.Fatalf("cast_info counter = %q, want %q", infos[0][0], "replacegraveyard")
	}
	if o := e.G.Obj(boltID); o != nil && o.CastFlags&state.FlagReplaceGraveyard != 0 {
		t.Fatalf("the flag leaked past the resting move: castflags=%x", o.CastFlags)
	}
	replayCheck(t, e, cfg)
}

// TestPlayReplaceGraveyardDoesNotLeak: the flag is per-SA provenance, never
// a Play-mode-wide default. Two shapes on the same cards must still rest in
// the graveyard:
//  1. an ORDINARY hand cast of the same Lightning Bolt;
//  2. a Play cast whose SA carries NO ReplaceGraveyard$ — Jace's
//     Mindseeker's FishyCast (mill five from the opponent, then cast a
//     remembered instant/sorcery for free), driven end to end on the real
//     corpus card.
func TestPlayReplaceGraveyardDoesNotLeak(t *testing.T) {
	reg := searchTestRegistry(t)

	// (1) the ordinary hand cast: funded {R}, cast at seat 1, resolves, and
	// the Bolt rests in the graveyard with no CastInfo at all (a plain cast
	// carries no flag and no X).
	e, cfg := gddEngine(t, reg)
	boltID := searchMoveByName(t, e, "Lightning Bolt", state.ZHand)
	addMana(t, e, 0, "R")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after funding: %+v, want priority", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == boltID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the Bolt: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		submitChoices(t, e, 0)
	}
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(boltID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the hand-cast Bolt rests in %v, want the graveyard", o)
	}
	if n := len(castInfosFor(e, boltID)); n != 0 {
		t.Fatalf("%d cast_info event(s) on an ordinary hand cast, want 0 (no flag, no X)", n)
	}
	replayCheck(t, e, cfg)

	// (2) Jace's Mindseeker: the ETB trigger mills five from seat 1's
	// library (the Bolt is parked on top), remembers them, and chains
	// FishyCast — a DB$ Play | ValidZone$ Graveyard,Exile |
	// Valid$ Instant.IsRemembered | WithoutManaCost$ True | Amount$ 1 with
	// NO ReplaceGraveyard$. The played Bolt resolves and must rest in the
	// graveyard.
	jace := searchCorpusCard(t, reg, "Jace's Mindseeker")
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck0 := []*cards.Card{jace}
	for len(deck0) < 40 {
		deck0 = append(deck0, mountain)
	}
	deck1 := []*cards.Card{searchCorpusCard(t, reg, "Lightning Bolt")}
	for len(deck1) < 40 {
		deck1 = append(deck1, mountain)
	}
	cfg2 := seatZeroStart(Config{Seed: 7403, Names: []string{"mindseeker", "opponent"},
		Decks: [][]*cards.Card{deck0, deck1}, Tokens: reg.Tokens})
	e2 := New(cfg2)
	e2.Advance()
	toMain1(t, e2)
	bolt2 := seatLibraryTop(t, e2, 1, "Lightning Bolt")
	searchMoveByName(t, e2, "Jace's Mindseeker", state.ZBattlefield)
	for i := 0; i < 40; i++ {
		d := e2.Pending()
		if d == nil {
			break
		}
		if o := e2.G.Obj(bolt2); o != nil && o.Zone == state.ZGraveyard &&
			(d.Kind == decision.KPriority || d.ResumeKind == "") && d.Kind != decision.KModes {
			break
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e2)
		case decision.KTarget:
			// The mill's target ask offers both seats; the scenario needs
			// the opponent (the mill that remembers the parked Bolt).
			idx := -1
			for _, o := range d.Options {
				if o.Player == 1 {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("no opponent target offered: %+v", d.Options)
			}
			submitChoices(t, e2, idx)
		case decision.KModes:
			if d.ResumeKind == "play" {
				submitChoices(t, e2, 0)
			} else {
				submitChoices(t, e2)
			}
		case decision.KChoose:
			// The opponent's end-of-turn hand-size discard: the scenario is
			// already settled (the Bolt rests), so answer it and stop.
			submitChoices(t, e2, 0)
			if o := e2.G.Obj(bolt2); o == nil || o.Zone != state.ZGraveyard {
				t.Fatalf("unexpected choose mid-scenario: %+v", d)
			}
			return
		default:
			t.Fatalf("unexpected decision %d: kind=%s resume=%q", i, d.Kind, d.ResumeKind)
		}
	}
	if o := e2.G.Obj(bolt2); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the rider-less Play's Bolt rests in %v, want the graveyard", o)
	}
	for _, info := range castInfosFor(e2, bolt2) {
		if strings.Contains(info[0], "replacegraveyard") {
			t.Fatalf("the rider-less Play stamped the flag: counter=%q", info[0])
		}
	}
	replayCheck(t, e2, cfg2)
}

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// This file pins the Horizon Explorer Lander-token defect at both ends of the architecture.
//
// The Lander token (Horizon Explorer's attack trigger, token script
// c_a_lander_sac_search) carries an activated ability whose cost is
// `{2}, {T}, Sacrifice this token` — mana + {T} + `Sac<1/CARDNAME>`, the
// self-sacrifice family (471 corpus card files carry the shape). The engine's
// float-then-cast model offers a battlefield ability only when the FLOATING
// pool can pay its mana, so an empty-pool priority window offers none of it —
// by design, and pinned here so nobody "fixes" the engine side. The rv2c
// server-side projection (PotentialActions priced against PotentialMana) is
// the bridge: the client's auto-pass reads it and stops the window, the seat
// floats the {2}, and the live offer appears.
//
// The fixture mints the REAL corpus token via a logged TokenCreate (never a
// fixture copy: the pin is the corpus script's own `Sac<1/CARDNAME>` cost)
// and places two fixture Mountains that each tap for {R}.

// landerMountainSrc is a fixture Mountain WITH an explicit AB$ Mana line.
// The test parser does not run ApplyIntrinsics (only the registry pipeline
// does), so a bare fixture Mountain has no mana ability and could never be
// tapped — the cost line must be spelled out.
const landerMountainSrc = "Name:Mountain\nTypes:Basic Land Mountain\n" +
	"A:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add {R}.\n" +
	"Oracle:{T}: Add {R}.\n"

// landerTokenKey is the corpus token script stem for Horizon Explorer's
// Lander token ("{2}, {T}, Sacrifice this token: Search your library for a
// basic land card, put it onto the battlefield tapped, then shuffle.").
const landerTokenKey = "c_a_lander_sac_search"

// landerFixture builds a 2-seat game at seat 0's turn-1 main1 with an empty
// floating pool, n real corpus Lander tokens on the battlefield (minted via
// logged TokenCreate, the event path a real trigger resolution takes), and
// two untapped Mountains that each tap for {R} — the feedback snapshot's
// shape (three tokens, two untapped red sources, no floating mana). The
// hand is cleared so no cast or land drop can mask the ability answer, and
// the land drop is spent for the same reason.
func landerFixture(t *testing.T, n int) (*Engine, []state.ObjID, []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	if reg.Tokens[landerTokenKey] == nil {
		t.Fatal("corpus token registry has no c_a_lander_sac_search for the Lander token")
	}
	cfg := seatZeroStart(Config{Seed: 11, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	e.G.Players[0].LandsPlayed = 1
	e.G.SetZone(state.ZHand, 0, nil)
	var landers []state.ObjID
	for i := 0; i < n; i++ {
		e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: landerTokenKey})
		e.pending = nil
		bf := e.G.Zone(state.ZBattlefield, 0)
		landers = append(landers, bf[len(bf)-1])
	}
	m1 := onBoard(t, e, 0, landerMountainSrc)
	m2 := onBoard(t, e, 0, landerMountainSrc)
	e.pending = nil
	e.Advance()
	return e, landers, []state.ObjID{m1, m2}
}

// landerAbilityOption returns the pending decision's "ability" option for the
// given source, if one is offered.
func landerAbilityOption(d *decision.Decision, src state.ObjID) (decision.Option, bool) {
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == src {
			return o, true
		}
	}
	return decision.Option{}, false
}

// landerActivateIndex returns the pending decision's tap-for-mana ("activate")
// option index for the given source, or -1.
func landerActivateIndex(d *decision.Decision, src state.ObjID) int {
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == src {
			return o.Index
		}
	}
	return -1
}

// TestLanderTokenAbilityOfferedOnlyWhenPoolFunded is the engine-offer pin
// (leaf 1): a battlefield token ability whose cost is mana + {T} +
// Sac<1/CARDNAME> is offered in an empty-pool priority window while the
// seat's untapped Mountains can fund the {2} (the autopay offer,
// legal_walk_autopay.go), still offered once one Mountain is tapped (the
// pool {R} plus the other Mountain pays), and choosing it from the empty
// pool taps both Mountains and puts the ability on the stack. The offer is
// the planner's exact witness, not "some mana exists": with a Mountain
// tapped away and the pool empty the {2} is short, so it is withheld.
func TestLanderTokenAbilityOfferedOnlyWhenPoolFunded(t *testing.T) {
	t.Parallel()
	e, landers, mounts := landerFixture(t, 1)
	lander := landers[0]
	m1, m2 := mounts[0], mounts[1]

	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision pending at main1")
	}
	if n := len(landers); n != 1 {
		t.Fatalf("fixture minted %d lander tokens, want 1", n)
	}
	o, ok := landerAbilityOption(d, lander)
	if !ok {
		t.Fatalf("empty-pool window withheld the Lander ability though two untapped Mountains fund {2}; options=%+v", d.Options)
	}
	if o.Ability != 0 {
		t.Errorf("offered ability index %d, want 0 (the token's ChangeZone is its only AB)", o.Ability)
	}

	// First tap: pool {R}; the other Mountain still funds the second {1}.
	a1 := landerActivateIndex(d, m1)
	if a1 < 0 {
		t.Fatalf("no activate option for Mountain %d; options=%+v", m1, d.Options)
	}
	submitChoices(t, e, a1)
	d = e.Pending()
	if d == nil {
		t.Fatal("no priority decision after the first tap")
	}
	if _, ok := landerAbilityOption(d, lander); !ok {
		t.Fatalf("pool {R} plus an untapped Mountain must fund the Lander ability; options=%+v", d.Options)
	}

	// Choosing it pays the rest from the second Mountain.
	o, _ = landerAbilityOption(d, lander)
	stack := len(e.G.Stack)
	submitChoices(t, e, o.Index)
	if !e.G.Obj(m2).Tapped || len(e.G.Stack) != stack+1 {
		t.Fatalf("autopay: Mountain %d tapped=%v, stack %d -> %d; want it tapped and the ability on the stack (pending %+v)",
			m2, e.G.Obj(m2).Tapped, stack, len(e.G.Stack), e.Pending())
	}
}

// TestLanderTokenIsAPotentialActionAfterTap is the projection pin (leaf 2)
// on the rv2c baseline: the potential-action walk — the server field the
// auto-pass stop decision reads — must carry the Lander ability when the
// seat's untapped sources can fund the {2} (the self-sacrifice part is
// always satisfiable: Sac<1/CARDNAME> resolves to the untapped source
// itself), for every one of the snapshot's THREE tokens; and must carry none
// of it once both Mountains are tapped, because the {2} can no longer be
// funded from anywhere. If nonManaCastable's Sac<1/CARDNAME> handling were
// reverted to fail closed, the funded half of this test fails — that is the
// shape the projection must keep admitting.
func TestLanderTokenIsAPotentialActionAfterTap(t *testing.T) {
	t.Parallel()
	e, landers, mounts := landerFixture(t, 3)
	if len(landers) != 3 {
		t.Fatalf("fixture minted %d lander tokens, want 3 (the snapshot's count)", len(landers))
	}

	// Funded potential: both Mountains untapped, pool empty — the exact
	// window the reporter sat in. Every token's search ability must be a
	// potential action, so the client's post-tap projection stops the window
	// and the seat can float the {2}.
	for _, tk := range landers {
		found := false
		var ab int
		for _, a := range e.PotentialActions(0) {
			if a.Kind == "ability" && a.Obj == tk {
				found, ab = true, a.Ability
			}
		}
		if !found {
			t.Fatalf("potential_actions carries no ability for Lander token %d — the auto-pass reads this field and would eat the window; actions=%+v", tk, e.PotentialActions(0))
		}
		if ab != 0 {
			t.Errorf("potential action for token %d names ability %d, want 0", tk, ab)
		}
	}

	// The walk must carry only real plays, never the kinds every priority
	// window offers anyway.
	for _, a := range e.PotentialActions(0) {
		switch a.Kind {
		case "cast", "ability", "play_land":
		default:
			t.Errorf("potential_actions carried kind %q (label %q) — never a play", a.Kind, a.Label)
		}
	}

	// Unfunded: both Mountains tapped, the pool can never reach {2} — the
	// walk must not carry the ability (a wrongly promised action would stop
	// windows forever).
	for _, m := range mounts {
		e.G.Obj(m).Tapped = true
	}
	e.staticEpoch = -1
	e.activeEpoch = -1
	for _, tk := range landers {
		for _, a := range e.PotentialActions(0) {
			if a.Kind == "ability" && a.Obj == tk {
				t.Errorf("both Mountains tapped: potential_actions still carries the Lander ability for token %d; nothing can fund the {2}", tk)
			}
		}
	}
}

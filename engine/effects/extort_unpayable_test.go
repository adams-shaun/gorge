package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// extortTapeHost serves the Extort ask the way the resolution kernel does: it
// records every decision posed and, on a "pay" answer, charges the {W/B} pip
// as rules' extortAnswerRecord does (the fake host has no answer record).
type extortTapeHost struct {
	*fakeHost
	posed []*decision.Decision
	pay   bool
}

func (h *extortTapeHost) AskCount() uint64 { return uint64(len(h.posed)) }

func (h *extortTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	h.posed = append(h.posed, d)
	if !h.pay {
		return decision.Intent{Choices: []int{1}}, true
	}
	pl := h.g.Players[d.Player]
	letter := "W"
	if pl.Pool[state.MW] == 0 {
		letter = "B"
	}
	h.Emit(events.Event{Kind: events.ManaAdd, Player: d.Player, Counter: letter, Amount: -1})
	return decision.Intent{Choices: []int{0}}, true
}

func extortLifeDelta(h *fakeHost) (opp, me int) {
	for _, ev := range h.log {
		if ev.Kind != events.LifeChange {
			continue
		}
		if ev.Player == 0 {
			me += int(ev.Amount)
		} else {
			opp += int(ev.Amount)
		}
	}
	return opp, me
}

// TestExtortAsksOnlyWhenThePipIsPayable: CR 118.3 -- a player can't pay a cost
// without the resources to pay it, so an Extort trigger resolving with no {W}
// or {B} in the pool poses no pay option and drains nothing, while one with a
// floating {W} (or {B}) still asks and drains on a pay.
func TestExtortAsksOnlyWhenThePipIsPayable(t *testing.T) {
	for _, tc := range []struct {
		name      string
		pool      func(*state.Player)
		restrict  []state.ManaRestriction
		wantAsked bool
		pay       bool
		wantDrain bool
	}{
		{name: "empty pool", wantAsked: false, pay: true},
		{name: "only red", pool: func(p *state.Player) { p.Pool[state.MR] = 2 }, wantAsked: false, pay: true},
		{name: "restricted white only",
			pool:     func(p *state.Player) { p.Pool[state.MW] = 1 },
			restrict: []state.ManaRestriction{{Color: "W", Amount: 1, Valid: "Spell.Creature"}}, wantAsked: false, pay: true},
		{name: "white pays", pool: func(p *state.Player) { p.Pool[state.MW] = 1 }, wantAsked: true, pay: true, wantDrain: true},
		{name: "black pays", pool: func(p *state.Player) { p.Pool[state.MB] = 1 }, wantAsked: true, pay: true, wantDrain: true},
		{name: "white declines", pool: func(p *state.Player) { p.Pool[state.MW] = 1 }, wantAsked: true, pay: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fh, c := fixtureHost(t)
			h := &extortTapeHost{fakeHost: fh, pay: tc.pay}
			if tc.pool != nil {
				tc.pool(&h.g.Players[0])
			}
			h.g.Players[0].RestrictedMana = tc.restrict
			effExtort(h, c, sa(t, "DB$ Extort"))
			if got := len(h.posed) > 0; got != tc.wantAsked {
				t.Fatalf("Extort ask posed = %v, want %v (posed %d)", got, tc.wantAsked, len(h.posed))
			}
			opp, me := extortLifeDelta(h.fakeHost)
			if tc.wantDrain && (opp != -1 || me != 1) {
				t.Fatalf("paid Extort drained opp %d / gained %d, want -1 / +1", opp, me)
			}
			if !tc.wantDrain && (opp != 0 || me != 0) {
				t.Fatalf("Extort drained opp %d / gained %d without a payment", opp, me)
			}
			if !tc.wantAsked && hasNoteContaining(fh, "no engine host") {
				t.Fatal("an unpayable Extort logged a no-host note: it must resolve silently")
			}
		})
	}
}

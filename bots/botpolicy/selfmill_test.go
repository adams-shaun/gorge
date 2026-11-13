package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// land is an untapped own land making one mana of colour slot c (WUBRG).
func selfMillLand(c int) Card {
	var p cards.ManaProduction
	p.Colour[c] = 1
	return Card{OnBattlefield: true, Land: true, Produces: p}
}

// TestSelfMillGuard: a name-keyed self-mill (Demonic Consultation's shape)
// is refused unless the seat wins on the empty library it leaves: a draw-
// replacement winner on the battlefield, or another castable winner its mana
// pays alongside the spell. An enters-trigger winner already on the
// battlefield (Thassa's Oracle, resolved) wins nothing.
func TestSelfMillGuard(t *testing.T) {
	const b, u = 2, 1
	consult := Card{NameKeyedSelfMill: true, ManaCost: "B", Castable: true, InstantSpeed: true}
	oracle := Card{EmptyLibraryWin: effects.EmptyLibraryWinETB, ManaCost: "U U", Castable: true, Creature: true, Power: 1}
	maniac := Card{EmptyLibraryWin: effects.EmptyLibraryWinDraw, ManaCost: "2 U", Creature: true, Power: 2}
	board := func(cs ...Card) Board {
		m := map[state.ObjID]Card{1: consult}
		for i, c := range cs {
			m[state.ObjID(i+2)] = c
		}
		return Board{IsMain: true, MyTurn: true, Cards: TableOf(m)}
	}
	cases := []struct {
		name   string
		b      Board
		vetoed bool
	}{
		{"no winner", board(selfMillLand(b), selfMillLand(u), selfMillLand(u)), true},
		{"oracle in hand, mana for both", board(oracle, selfMillLand(b), selfMillLand(u), selfMillLand(u)), false},
		{"oracle in hand, mana for one", board(oracle, selfMillLand(b), selfMillLand(u)), true},
		{"oracle in hand, wrong colours", board(oracle, selfMillLand(b), selfMillLand(b), selfMillLand(b)), true},
		{"oracle resolved on the battlefield", board(func() Card { c := oracle; c.Castable, c.OnBattlefield = false, true; return c }(), selfMillLand(b)), true},
		{"maniac on the battlefield", board(func() Card { c := maniac; c.OnBattlefield = true; return c }(), selfMillLand(b)), false},
	}
	for _, tc := range cases {
		if got := tc.b.selfMillVeto(consult); got != tc.vetoed {
			t.Errorf("%s: vetoed %v, want %v", tc.name, got, tc.vetoed)
		}
	}
	// The cast ranking honours it: the vetoed self-mill is never the cast,
	// while the same card without the fact is.
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "cast", Obj: 1}}}
	plain := consult
	plain.NameKeyedSelfMill = false
	bp := board(selfMillLand(b))
	bp.Cards = TableOf(map[state.ObjID]Card{1: plain, 2: selfMillLand(b)})
	if got := withTuned(bp, func(w *CastWeights) { w.CastThreshold = -1000 }).chooseCast(d); got != 0 {
		t.Fatalf("plain card: chooseCast = %d, want 0 (the control must cast it)", got)
	}
	bv := board(selfMillLand(b))
	if got := withTuned(bv, func(w *CastWeights) { w.CastThreshold = -1000 }).chooseCast(d); got != -1 {
		t.Fatalf("self-mill with no winner: chooseCast = %d, want -1", got)
	}
}

// TestPayOrLoseNeverDeclined: an unless-pay whose unpaid branch loses the
// game (Summoner's Pact's upkeep trigger) is paid even facing lethal board
// damage; an ordinary unless-pay is still declined there.
func TestPayOrLoseNeverDeclined(t *testing.T) {
	b := Board{Life: TableOf(map[state.PlayerID]int32{0: 3, 1: 20}),
		Creatures: TableOf(map[state.ObjID]Creature{9: {Controller: 1, Power: 5}})}
	ask := func(sa *cards.SA) *decision.Decision {
		return &decision.Decision{Player: 0, Kind: decision.KModes, ResumeKind: "unless_pay", ResumeSA: sa,
			Options: []decision.Option{{Index: 0, Mode: decision.ModeUnlessPay}, {Index: 1, Mode: decision.ModeUnlessDecline}}}
	}
	lose := &cards.SA{API: "LosesGame", Params: map[string]string{"UnlessCost": "2 G G", "UnlessPayer": "You", "Defined": "You"}}
	if got := b.unlessManaPayOffer(ask(lose)); len(got) != 1 || got[0] != 0 {
		t.Fatalf("pay-or-lose facing lethal: %v, want [0] (pay)", got)
	}
	tax := &cards.SA{API: "Counter", Params: map[string]string{"UnlessCost": "3"}}
	if got := b.unlessManaPayOffer(ask(tax)); len(got) != 1 || got[0] != 1 {
		t.Fatalf("ordinary unless-pay facing lethal: %v, want [1] (decline)", got)
	}
}

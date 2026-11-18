package rules

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// assignGroupGame seats n players; seat 0 holds spell (cast with mana) and the
// other decks are filler built from the corpus cards named in fill. Every seat
// then has each card of onBattlefield[seat] moved from its deck onto the
// battlefield. The spell is cast and resolution is driven to its first
// non-priority decision, which is returned with the engine.
func assignGroupGame(t *testing.T, n int, spell, mana string, fill []string, onBattlefield map[state.PlayerID][]string) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	mk := func(first string) []*cards.Card {
		var deck []*cards.Card
		if first != "" {
			deck = append(deck, searchCorpusCard(t, reg, first))
		}
		for _, names := range onBattlefield {
			for _, nm := range names {
				deck = append(deck, searchCorpusCard(t, reg, nm))
			}
		}
		for len(deck) < 40 {
			deck = append(deck, searchCorpusCard(t, reg, fill[len(deck)%len(fill)]))
		}
		return deck
	}
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	for i := range names {
		names[i] = "p" + strconv.Itoa(i)
		first := ""
		if i == 0 {
			first = spell
		}
		decks[i] = mk(first)
	}
	cfg := seatZeroStart(Config{Seed: 4417, Names: names, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := searchMoveByName(t, e, spell, state.ZHand)
	for p, list := range onBattlefield {
		for _, nm := range list {
			searchMoveByNameSeat(t, e, p, nm, state.ZBattlefield)
		}
	}
	addMana(t, e, 0, mana)
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %s: %+v", spell, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilNonPriority(t, e, 40)
	return e, cfg, id
}

// assignGroupAnswer answers the pending friend-or-foe asks: one per player in
// seat order, each naming that seat's group (true = Friend). It returns when
// the next pending decision is not an assigngroup ask.
func assignGroupAnswer(t *testing.T, e *Engine, friend []bool) {
	t.Helper()
	for i, f := range friend {
		d := e.Pending()
		if d == nil || d.ResumeKind != "assigngroup" || d.Kind != decision.KChoose {
			t.Fatalf("ask %d: pending %+v, want an assigngroup KChoose", i, d)
		}
		if d.Player != 0 || len(d.Options) != 2 || d.Options[0].Player != state.PlayerID(i) ||
			d.Options[0].Label != "Friend" || d.Options[1].Label != "Foe" {
			t.Fatalf("ask %d = player %d options %+v, want the caster choosing Friend/Foe for seat %d", i, d.Player, d.Options, i)
		}
		pick := 1
		if f {
			pick = 0
		}
		submitChoices(t, e, d.Options[pick].Index)
	}
}

// assignGroupFinish answers every remaining non-priority decision with its
// first option (a search takes the first land, a sacrifice the first
// permanent) until priority returns.
func assignGroupFinish(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			return
		}
		if len(d.Options) == 0 || d.Min == 0 && len(d.Options) == 0 {
			t.Fatalf("unanswerable decision %+v", d)
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	t.Fatal("resolution did not finish")
}

func battlefieldCount(e *Engine, p state.PlayerID, name string) (n, tapped int) {
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			n++
			if o.Tapped {
				tapped++
			}
		}
	}
	return n, tapped
}

// TestEDHAssignGroupPirsWhimFourSeats plays Pir's Whim at four seats: the
// caster marks seats 0 and 1 friend and seats 2 and 3 foe. Each friend fetches
// a land tapped; each foe sacrifices an artifact or enchantment.
func TestEDHAssignGroupPirsWhimFourSeats(t *testing.T) {
	t.Parallel()
	bf := map[state.PlayerID][]string{1: {"Sol Ring"}, 2: {"Sol Ring"}, 3: {"Sol Ring"}}
	e, cfg, whim := assignGroupGame(t, 4, "Pir's Whim", "GGGG", []string{"Forest"}, bf)
	for p := state.PlayerID(1); p < 4; p++ {
		if n, _ := battlefieldCount(e, p, "Sol Ring"); n != 1 {
			t.Fatalf("precondition: seat %d has %d Sol Ring, want 1", p, n)
		}
	}
	before := [4]int{}
	for p := range before {
		before[p] = len(e.G.Zone(state.ZBattlefield, state.PlayerID(p)))
	}
	assignGroupAnswer(t, e, []bool{true, true, false, false})
	assignGroupFinish(t, e)
	if z := e.G.Obj(whim).Zone; z != state.ZGraveyard {
		t.Fatalf("Pir's Whim zone = %s, want the graveyard", z)
	}
	for _, p := range []state.PlayerID{0, 1} { // friends: a tapped land, nothing sacrificed
		if got := len(e.G.Zone(state.ZBattlefield, p)); got != before[p]+1 {
			t.Errorf("friend seat %d battlefield %d -> %d, want +1", p, before[p], got)
		}
		if n, tp := battlefieldCount(e, p, "Forest"); n < 1 || tp != n {
			t.Errorf("friend seat %d Forests %d tapped %d, want a tapped fetched land", p, n, tp)
		}
	}
	if n, _ := battlefieldCount(e, 1, "Sol Ring"); n != 1 {
		t.Errorf("friend seat 1 Sol Rings = %d, want 1 (friends sacrifice nothing)", n)
	}
	for _, p := range []state.PlayerID{2, 3} { // foes: the artifact is sacrificed, no land
		if n, _ := battlefieldCount(e, p, "Sol Ring"); n != 0 {
			t.Errorf("foe seat %d Sol Rings = %d, want 0 (sacrificed)", p, n)
		}
		if n, _ := battlefieldCount(e, p, "Forest"); n != 0 {
			t.Errorf("foe seat %d fetched a land (%d Forests)", p, n)
		}
	}
	replayCheck(t, e, cfg)
}

func assignGroupHand(e *Engine, p state.PlayerID) int { return len(e.G.Zone(state.ZHand, p)) }

// TestEDHAssignGroupKhorvathsFuryFriendDrawsFoeTakesDamage: seat 0 is a friend
// (discards its hand, draws that many plus one), seats 1 and 2 are foes and
// take damage equal to the cards in their hands.
func TestEDHAssignGroupKhorvathsFuryFriendDrawsFoeTakesDamage(t *testing.T) {
	t.Parallel()
	e, cfg, fury := assignGroupGame(t, 3, "Khorvath's Fury", "RRRRR", []string{"Mountain"}, nil)
	h0, h1, h2 := assignGroupHand(e, 0), assignGroupHand(e, 1), assignGroupHand(e, 2)
	l1, l2, l0 := e.G.Players[1].Life, e.G.Players[2].Life, e.G.Players[0].Life
	if h0 < 2 || h1 < 2 || h2 < 2 {
		t.Fatalf("precondition: hands %d/%d/%d too small to tell the groups apart", h0, h1, h2)
	}
	assignGroupAnswer(t, e, []bool{true, false, false})
	assignGroupFinish(t, e)
	if z := e.G.Obj(fury).Zone; z != state.ZGraveyard {
		t.Fatalf("Khorvath's Fury zone = %s, want the graveyard", z)
	}
	if got := assignGroupHand(e, 0); got != h0+1 {
		t.Errorf("friend hand %d -> %d, want the discarded %d cards redrawn plus one", h0, got, h0)
	}
	if got := e.G.Players[0].Life; got != l0 {
		t.Errorf("friend life %d -> %d, want unchanged", l0, got)
	}
	if got := l1 - e.G.Players[1].Life; int(got) != h1 {
		t.Errorf("foe 1 took %d damage, want its %d cards in hand", got, h1)
	}
	if got := l2 - e.G.Players[2].Life; int(got) != h2 {
		t.Errorf("foe 2 took %d damage, want its %d cards in hand", got, h2)
	}
	replayCheck(t, e, cfg)
}

// TestEDHAssignGroupRegnasSanctionFriendCountersFoeKeepsOne: the friend puts a
// +1/+1 counter on each creature it controls; the foe chooses one untapped
// creature and taps the rest.
func TestEDHAssignGroupRegnasSanctionFriendCountersFoeKeepsOne(t *testing.T) {
	t.Parallel()
	bf := map[state.PlayerID][]string{
		0: {"Grizzly Bears", "Grizzly Bears"},
		1: {"Grizzly Bears", "Grizzly Bears", "Grizzly Bears"},
	}
	e, cfg, _ := assignGroupGame(t, 2, "Regna's Sanction", "WWWW", []string{"Plains"}, bf)
	if n, _ := battlefieldCount(e, 0, "Grizzly Bears"); n != 2 {
		t.Fatalf("precondition: friend has %d Bears, want 2", n)
	}
	if n, tp := battlefieldCount(e, 1, "Grizzly Bears"); n != 3 || tp != 0 {
		t.Fatalf("precondition: foe has %d Bears (%d tapped), want 3 untapped", n, tp)
	}
	assignGroupAnswer(t, e, []bool{true, false})
	assignGroupFinish(t, e)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == "Grizzly Bears" && o.Counter("P1P1") != 1 {
			t.Errorf("friend Bears %d has %d +1/+1 counters, want 1", id, o.Counter("P1P1"))
		}
	}
	n, tp := battlefieldCount(e, 1, "Grizzly Bears")
	if n != 3 || tp != 2 {
		t.Errorf("foe Bears %d with %d tapped, want 3 with exactly 2 tapped (one kept untapped)", n, tp)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == "Grizzly Bears" && o.Counter("P1P1") != 0 {
			t.Errorf("foe Bears %d got a counter", id)
		}
	}
	replayCheck(t, e, cfg)
}

// TestEDHAssignGroupZndrspltsJudgmentFriendCopiesFoeBounces: the friend creates
// a token copy of a creature it controls; the foe returns a creature it
// controls to its owner's hand.
func TestEDHAssignGroupZndrspltsJudgmentFriendCopiesFoeBounces(t *testing.T) {
	t.Parallel()
	bf := map[state.PlayerID][]string{0: {"Grizzly Bears"}, 1: {"Grizzly Bears"}}
	e, cfg, _ := assignGroupGame(t, 2, "Zndrsplt's Judgment", "UUUUU", []string{"Island"}, bf)
	if n, _ := battlefieldCount(e, 0, "Grizzly Bears"); n != 1 {
		t.Fatalf("precondition: friend has %d Bears, want 1", n)
	}
	foeHand := assignGroupHand(e, 1)
	assignGroupAnswer(t, e, []bool{true, false})
	assignGroupFinish(t, e)
	if n, _ := battlefieldCount(e, 0, "Grizzly Bears"); n != 2 {
		t.Errorf("friend Bears = %d, want the original and a token copy", n)
	}
	if n, _ := battlefieldCount(e, 1, "Grizzly Bears"); n != 0 {
		t.Errorf("foe Bears = %d, want 0 (returned to hand)", n)
	}
	if got := assignGroupHand(e, 1); got != foeHand+1 {
		t.Errorf("foe hand %d -> %d, want +1", foeHand, got)
	}
	replayCheck(t, e, cfg)
}

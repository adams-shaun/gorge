package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Task tgtplayer1 end-to-end pins on the REAL corpus cards (built from the
// compiled corpus only -- no Forge script text is committed here, per the
// licensing rule). Both carriers were silent zeros before the
// TargetedPlayer$ count arm: Knollspine Dragon drew nothing, The Mouth of
// Sauron always amassed 0.

// tgtplayerTestEngine deals seat 0 a 40-card corpus deck whose first card is
// first, then eight Forests and Grizzly Bears filler; seat 1's deck is
// opener (its first two cards) then Mountains. Seat 0 is guaranteed the
// starting player so the fixtures address seats by index.
func tgtplayerTestEngine(t *testing.T, reg *cards.Registry, first, extra0, opener1, opener2 string) (*Engine, Config) {
	t.Helper()
	forest := searchCorpusCard(t, reg, "Forest")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	mountain := searchCorpusCard(t, reg, "Mountain")
	// Seat 0's deck gains Shock as its second card: an instant for the
	// caster's own graveyard discriminator.
	deck := []*cards.Card{searchCorpusCard(t, reg, first), searchCorpusCard(t, reg, extra0)}
	for i := 0; i < 8; i++ {
		deck = append(deck, forest)
	}
	for len(deck) < 40 {
		deck = append(deck, bear)
	}
	opp := []*cards.Card{searchCorpusCard(t, reg, opener1), searchCorpusCard(t, reg, opener2)}
	for len(opp) < 40 {
		opp = append(opp, mountain)
	}
	cfg := seatZeroStart(Config{Seed: 9207, Names: []string{"caster", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// moveGraveyardByName finds the NAMED card in seat p's hand/library and
// moves it to seat p's graveyard with one logged MoveZone (the searchMoveByName
// shape, generalized to the seat and destination).
func moveGraveyardByName(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZGraveyard})
				e.pending = nil
				e.priorityRound()
				return id
			}
		}
	}
	t.Fatalf("corpus card %q absent from seat %d's hand/library", name, p)
	return 0
}

// TestKnollspineDragonDrawsTheDamageDealtToTargetOpponent pins the whole
// chain end to end: the ETB trigger's optional discard is accepted, the
// ValidTgts$ Opponent target of the depth-2 DBDraw sub is ASKED (the
// chosenTargetsFor guard fix -- before it, the blanket Defined$ suppression
// dropped the ask and TargetedPlayer$DamageThisTurn read an empty target
// list), and the draw equals the damage dealt to that opponent this turn.
// Two different damage values across the cases: the defect was a silent
// zero, so a single value could pass by coincidence.
func TestKnollspineDragonDrawsTheDamageDealtToTargetOpponent(t *testing.T) {
	for _, dmg := range []int32{4, 3} {
		t.Run("damage", func(t *testing.T) {
			reg := searchTestRegistry(t)
			e, _ := tgtplayerTestEngine(t, reg, "Knollspine Dragon", "Shock", "Mountain", "Mountain")
			// Seat 0 dealt seat 1 exactly dmg damage this turn (a player hit
			// is Kind Damage with Player set and Obj 0; the fold also drops
			// its life).
			e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: dmg})
			if got := e.DamageTakenThisTurn(1); got != dmg {
				t.Fatalf("DamageTakenThisTurn(1) = %d, want %d", got, dmg)
			}
			if got := e.DamageTakenThisTurn(0); got != 0 {
				t.Fatalf("DamageTakenThisTurn(0) = %d, want 0", got)
			}
			addMana(t, e, 0, "RRRRRRR")
			id := searchMoveByName(t, e, "Knollspine Dragon", state.ZHand)
			d := castFixture(t, e, id, -1)
			if d == nil || d.Kind != decision.KTriggerOptional {
				t.Fatalf("after casting: %+v, want the optional trigger ask", d)
			}
			n0 := len(e.L.Events)
			submitChoices(t, e, 0) // accept the "you may discard your hand" trigger
			d = e.Pending()
			if d == nil || d.Kind != decision.KChoose {
				t.Fatalf("after accepting: %+v, want the DBDraw target ask", d)
			}
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "player" && o.Player == 1 {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("seat 1 not offered as the DBDraw target: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			passUntilStackEmpty(t, e, 20)
			// The whole hand was discarded, then exactly dmg cards were drawn.
			discards, draws := 0, 0
			for _, ev := range e.L.Events[n0:] {
				switch {
				case ev.Kind == events.Draw && ev.Player == 0:
					draws++
				case events.IsDiscard(ev) && ev.Player == 0:
					discards++
				}
			}
			if discards == 0 {
				t.Errorf("no discard events recorded: %+v", e.L.Events[n0:])
			}
			if draws != int(dmg) {
				t.Errorf("drew %d cards, want %d (the damage dealt to the asked opponent)", draws, dmg)
			}
			if got := len(e.G.Zone(state.ZHand, 0)); got != int(dmg) {
				t.Errorf("hand after discard-and-draw = %d cards, want %d", got, dmg)
			}
		})
	}
}

// TestMouthOfSauronAmassCountsTheMilledPlayerGraveyard pins the head's
// perspective: `TargetedPlayer$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn`
// counts the MILLED TARGET player's own instants/sorceries in THEIR
// graveyard. Seat 0's own graveyard carries an instant of its own (Shock),
// so a wrong perspective (You = the resolving controller) would read 3.
// (Ownership never crosses players in a zone -- the zone lists are keyed by
// the card's owner -- so the YouOwn filter's live discriminator is exactly
// this perspective split.)
func TestMouthOfSauronAmassCountsTheMilledPlayerGraveyard(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := tgtplayerTestEngine(t, reg, "The Mouth of Sauron", "Shock", "Giant Growth", "Rampant Growth")
	// Seat 1's graveyard: its own instant and sorcery. Seat 0's graveyard:
	// its own instant, which must NOT count.
	moveGraveyardByName(t, e, 1, "Giant Growth")
	moveGraveyardByName(t, e, 1, "Rampant Growth")
	moveGraveyardByName(t, e, 0, "Shock")
	addMana(t, e, 0, "UUUBBBB")
	id := searchMoveByName(t, e, "The Mouth of Sauron", state.ZHand)
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after casting: %+v, want the trigger's target ask", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seat 1 not offered as the mill target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	// The mill moved 3 Mountains into seat 1's graveyard.
	milled := 0
	for _, id2 := range e.G.Zone(state.ZGraveyard, 1) {
		o := e.G.Obj(id2)
		if o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			milled++
		}
	}
	if milled != 3 {
		t.Errorf("milled %d Mountains, want 3", milled)
	}
	// Amass X where X = the instants/sorceries in the MILLED player's
	// graveyard: the Army token carries exactly X +1/+1 counters.
	army, counters := state.ObjID(0), int32(-1)
	for _, id2 := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id2)
		if o == nil || o.Face() == nil {
			continue
		}
		for _, typ := range o.Face().Types {
			if typ == "Army" {
				army, counters = id2, o.Counter("P1P1")
				break
			}
		}
		if army != 0 {
			break
		}
	}
	if army == 0 {
		t.Fatalf("no Army token was amassed")
	}
	if counters != 2 {
		t.Errorf("Army amassed %d counters, want 2 (Giant Growth + Rampant Growth; Shock in the CASTER'S graveyard does not count)", counters)
	}
}

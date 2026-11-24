package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// lifeIdentTable is vanilla's table with War Room's draw ability on a land
// ("{3}, {T}, Pay life equal to the number of colors in your commanders'
// color identity: Draw a card. Activate only in a game of Commander") and
// three fixture commanders: a colourless one, a white-black one and a
// black-green one.
func lifeIdentTable() (tbl *kop.Table, room, colourless, wb, bg CardID) {
	t, _, ids := newModeTable(
		kop.CardDef{Name: "Test War Room", Types: kop.TLand, Abilities: []kop.Ability{{Kind: kop.AbActivated, CmdOnly: true,
			Cost: kop.Cost{Mana: kop.ManaCost{Generic: 3}, Flags: kop.CostTap, LifeIdent: true},
			Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Colourless Commander", Cost: kop.ManaCost{Generic: 2}, Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test WB Commander", Cost: kop.ManaCost{Generic: 2}, Types: kop.TCreature, Power: 2, Toughness: 2, Identity: kop.ColW | kop.ColB},
		kop.CardDef{Name: "Test BG Commander", Cost: kop.ManaCost{Generic: 2}, Types: kop.TCreature, Power: 2, Toughness: 2, Identity: kop.ColB | kop.ColG},
	)
	return t, ids[0], ids[1], ids[2], ids[3]
}

// lifeIdentScen deals a two-seat game whose seat 0 has the commanders cmds0
// (one, or a partner pair) and whose seat 1 has cmd1, empties the hands, and
// gives each seat an untapped War Room and three Forests.
func lifeIdentScen(t *testing.T, commander bool, cmds0 []CardID, cmd1 CardID) (sc *scen, rooms [2]Obj) {
	t.Helper()
	tbl, room, _, _, _ := lifeIdentTable()
	var decks [MaxSeats][]CardID
	cs := noCommanders()
	for p, cmds := range [][]CardID{cmds0, {cmd1}} {
		d := make([]CardID, 20)
		for i := range d {
			d[i] = v.Forest
		}
		for j, c := range cmds {
			d[j] = c
			cs[p][j] = int16(j)
		}
		decks[p] = d
	}
	s := new(State)
	s.InitN(tbl, decks, 2, 1, Config{NoElide: true, Commander: commander, Commanders: cs})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	sc = &scen{s: s, k: s.k()}
	for p := uint8(0); p < 2; p++ {
		rooms[p] = sc.perm(room, p)
		sc.lands(p, v.Forest, 3)
	}
	return sc, rooms
}

// lifeIdentTurn gives seat p the turn and priority in its first main phase.
func lifeIdentTurn(sc *scen, p uint8) {
	sc.s.active, sc.s.prio, sc.s.passes, sc.s.step, sc.s.ctl = p, p, 0, kop.StepMain1, ctlPriority
}

// lifeIdentActivate activates room for the seat holding priority, answers
// the payment with the first option each time, and resolves the ability.
func lifeIdentActivate(t *testing.T, sc *scen, room Obj) {
	t.Helper()
	sc.do(t, AkActivate, TCode(room), scAny)
	for i := 0; i < 20 && sc.s.ctl != ctlPriority; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			t.Fatalf("decision kind %d with no options while activating", d.Kind)
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.sp != 1 {
		t.Fatalf("stack %d, want the draw ability on it", sc.s.sp)
	}
	sc.passBoth(t)
}

// TestLifeIdentChargesTheColourCount: the draw ability charges its activator
// the number of colours in that player's commanders' colour identity (CR
// 903.4): 2 for a white-black commander, 3 for a white-black and black-green
// partner pair (black counts once), 0 for a colourless commander. Each
// activation taps the land and three more for {3}, and draws one card.
func TestLifeIdentChargesTheColourCount(t *testing.T) {
	_, _, colourless, wb, bg := lifeIdentTable()
	for _, tc := range []struct {
		name string
		cmds []CardID
		want int16
	}{
		{"colourless", []CardID{colourless}, 0},
		{"two colours", []CardID{wb}, 2},
		{"partners sharing a colour", []CardID{wb, bg}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, rooms := lifeIdentScen(t, true, tc.cmds, colourless)
			if got := sc.s.lifeIdentOwed(sc.k, 0); got != int(tc.want) {
				t.Fatalf("seat 0 owes %d life, want %d", got, tc.want)
			}
			if !sc.activateOffered(rooms[0], 0) {
				t.Fatal("the draw ability is not offered in a Commander game")
			}
			life, hand := sc.s.pl[0].life, sc.s.pl[0].handN
			lifeIdentActivate(t, sc, rooms[0])
			if got := life - sc.s.pl[0].life; got != tc.want {
				t.Fatalf("the activation cost %d life, want %d", got, tc.want)
			}
			if sc.s.pl[0].handN != hand+1 || !sc.tapped(rooms[0]) {
				t.Fatalf("hand %d (want %d), land tapped %v", sc.s.pl[0].handN, hand+1, sc.tapped(rooms[0]))
			}
			if sc.s.pl[1].life != 40 {
				t.Fatalf("seat 1's life %d, want 40: the activator pays", sc.s.pl[1].life)
			}
		})
	}
}

// TestLifeIdentPayerIsTheActivator: each seat pays for its own commanders.
// Seat 1's commander is colourless, so its activation is free of life
// whatever seat 0's commanders are.
func TestLifeIdentPayerIsTheActivator(t *testing.T) {
	_, _, colourless, wb, bg := lifeIdentTable()
	sc, rooms := lifeIdentScen(t, true, []CardID{wb, bg}, colourless)
	lifeIdentTurn(sc, 1)
	if !sc.activateOffered(rooms[1], 0) {
		t.Fatal("the draw ability is not offered to seat 1")
	}
	lifeIdentActivate(t, sc, rooms[1])
	if sc.s.pl[0].life != 40 || sc.s.pl[1].life != 40 {
		t.Fatalf("life %d/%d, want 40/40: a colourless identity pays no life", sc.s.pl[0].life, sc.s.pl[1].life)
	}
}

// TestLifeIdentWithheldBelowTheCount: a player can pay life only out of a
// total at least that large (CR 119.4), so the ability is offered at exactly
// the count and withheld below it. A count of 0 is payable at any life total
// (CR 119.4b).
func TestLifeIdentWithheldBelowTheCount(t *testing.T) {
	_, _, colourless, wb, bg := lifeIdentTable()
	sc, rooms := lifeIdentScen(t, true, []CardID{wb, bg}, colourless)
	sc.s.pl[0].life = 2
	if sc.activateOffered(rooms[0], 0) {
		t.Fatal("offered at 2 life with a cost of 3")
	}
	sc.s.pl[0].life = 3
	if !sc.activateOffered(rooms[0], 0) {
		t.Fatal("withheld at 3 life with a cost of 3")
	}
	sc.s.pl[1].life = 1
	lifeIdentTurn(sc, 1)
	if !sc.activateOffered(rooms[1], 0) {
		t.Fatal("a cost of 0 life withheld at 1 life")
	}
}

// TestLifeIdentOnlyInCommander: outside a game of Commander the ability is
// never offered (gorge activationGameTypesOK). Without the gate it would be
// a free draw there, since no seat has a commander.
func TestLifeIdentOnlyInCommander(t *testing.T) {
	_, _, colourless, wb, _ := lifeIdentTable()
	sc, rooms := lifeIdentScen(t, false, []CardID{wb}, colourless)
	if sc.s.lifeIdentOwed(sc.k, 0) != 0 {
		t.Fatal("a seat with no commander owes life")
	}
	for p := uint8(0); p < 2; p++ {
		lifeIdentTurn(sc, p)
		if sc.activateOffered(rooms[p], 0) {
			t.Fatalf("offered to seat %d outside a Commander game", p)
		}
	}
}

// TestLifeIdentMutation: with DebugLifeIdentFree the part charges nothing,
// which the lockstep mutation test must see as a divergence from gorge.
func TestLifeIdentMutation(t *testing.T) {
	_, _, colourless, wb, _ := lifeIdentTable()
	sc, rooms := lifeIdentScen(t, true, []CardID{wb}, colourless)
	DebugLifeIdentFree = true
	defer func() { DebugLifeIdentFree = false }()
	lifeIdentActivate(t, sc, rooms[0])
	if sc.s.pl[0].life != 40 {
		t.Fatalf("life %d under the mutation, want 40", sc.s.pl[0].life)
	}
}

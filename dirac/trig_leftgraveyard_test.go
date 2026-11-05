package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newLeftGraveyardScen is newBLBScen over the pool: a matchup of deck vs deck,
// libraries of the table's Plains.
func newLeftGraveyardScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "graveleaves")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in this table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	pl, ok := id["Plains"]
	if !ok {
		t.Fatal("the scenario deck needs Plains")
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = pl
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// trigLeftgraveyardGraveAb is card c's "one or more cards leave your graveyard" trigger.
func trigLeftgraveyardGraveAb(t *testing.T, sc *scen, c CardID) uint8 {
	t.Helper()
	for j, a := range sc.k.t.Cards[c].Abilities {
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigLeaves && a.Trig.From == kop.ZoneGraveyard {
			return uint8(j)
		}
	}
	t.Fatalf("%s has no graveyard-leaves trigger", sc.k.t.Cards[c].Name)
	return 0
}

// TestGraveLeavesBatch: Chalk Outline's "whenever one or more creature
// cards leave your graveyard" queues once per move outside a batch, once
// for two moves inside one batch, and not for a noncreature card or the
// opponent's creature.
func TestGraveLeavesBatch(t *testing.T) {
	sc, id := newLeftGraveyardScen(t, []cardset.Entry{
		{Name: "Chalk Outline", Count: 1}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 4}, {Name: "Plains", Count: 8},
	})
	chalk, bears, forest := id["Chalk Outline"], id["Grizzly Bears"], id["Forest"]
	c := sc.perm(chalk, 0)
	ab := trigLeftgraveyardGraveAb(t, sc, chalk)
	grave := sc.grave(bears, 0, 2)
	sc.s.npend = 0
	sc.s.detach(grave[0])
	if n := batchPend(sc, c, ab); n != 1 {
		t.Fatalf("one creature leaving queued %d triggers, want 1", n)
	}
	sc.s.npend = 0
	old := sc.s.batchOpen()
	sc.s.detach(grave[0])
	sc.s.detach(grave[1])
	sc.s.batchClose(old)
	if n := batchPend(sc, c, ab); n != 1 {
		t.Fatalf("two creatures leaving in one batch queued %d triggers, want 1", n)
	}
	sc.s.npend = 0
	sc.s.detach(sc.grave(forest, 0, 1)[0])
	if n := batchPend(sc, c, ab); n != 0 {
		t.Fatalf("a land leaving queued %d triggers, want 0", n)
	}
	sc.s.npend = 0
	sc.s.detach(sc.grave(bears, 1, 1)[0])
	if n := batchPend(sc, c, ab); n != 0 {
		t.Fatalf("the opponent's creature leaving queued %d triggers, want 0", n)
	}
}

// TestGraveLeavesYourTurn: Attuned Hunter's line is limited to its
// controller's turn.
func TestGraveLeavesYourTurn(t *testing.T) {
	sc, id := newLeftGraveyardScen(t, []cardset.Entry{
		{Name: "Attuned Hunter", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 4}, {Name: "Plains", Count: 8},
	})
	hunter, bears := id["Attuned Hunter"], id["Grizzly Bears"]
	h := sc.perm(hunter, 0)
	ab := trigLeftgraveyardGraveAb(t, sc, hunter)
	grave := sc.grave(bears, 0, 2)
	sc.s.active = 1
	sc.s.npend = 0
	sc.s.detach(grave[0])
	if n := batchPend(sc, h, ab); n != 0 {
		t.Fatalf("on the opponent's turn queued %d triggers, want 0", n)
	}
	sc.s.active = 0
	sc.s.npend = 0
	sc.s.detach(grave[1])
	if n := batchPend(sc, h, ab); n != 1 {
		t.Fatalf("on its controller's turn queued %d triggers, want 1", n)
	}
}

// TestGraveSweepIsOneBatch: a graveyard sweep onto the battlefield through
// execChangeZoneAll opens the batch (gorge's effChangeZoneAll opens its zone
// batch for an entry), so a two-card move queues one trigger, and the
// sweep leaves the batch closed. A sweep to another zone is one event too:
// TestGraveSweepToExileIsOneEvent.
func TestGraveSweepIsOneBatch(t *testing.T) {
	sc, id := newLeftGraveyardScen(t, []cardset.Entry{
		{Name: "Chalk Outline", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 4}, {Name: "Plains", Count: 8},
	})
	chalk, bears := id["Chalk Outline"], id["Grizzly Bears"]
	c := sc.perm(chalk, 0)
	ab := trigLeftgraveyardGraveAb(t, sc, chalk)
	sc.grave(bears, 0, 2)
	var f kop.FilterID
	for i, fl := range sc.k.t.Filters {
		if fl.Ctrl == kop.CtrlYou && fl.Zone == kop.ZoneGraveyard && fl.Types == 0 && fl.Or == 0 {
			f = kop.FilterID(i)
			break
		}
	}
	if f == 0 {
		t.Fatal("no graveyard filter in the pool")
	}
	in := kop.Instr{Op: kop.OpChangeZoneAll, C: int16(kop.ZoneBattlefield), F: f}
	sc.s.npend = 0
	sc.s.res.e.ctrl = 0
	sc.s.execChangeZoneAll(sc.k, &in, false)
	if n := batchPend(sc, c, ab); n != 1 {
		t.Fatalf("a two-card sweep queued %d triggers, want 1", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("batch state left open: %#x", sc.s.g.batchLatch)
	}
}

// TestGraveLeavesCarriers: every class carrier queues its graveyard-leaves trigger
// for one matching card leaving its controller's graveyard.
func TestGraveLeavesCarriers(t *testing.T) {
	cases := []struct{ carrier, card string }{
		{"Attuned Hunter", "Forest"},
		{"Chalk Outline", "Grizzly Bears"},
		{"Cyan, Vengeful Samurai", "Grizzly Bears"},
		{"Desecrated Tomb", "Grizzly Bears"},
		{"Fang, Fearless l'Cie", "Forest"},
		{"Fuming Effigy", "Forest"},
		{"Garrison Excavator", "Forest"},
		{"Hardened Academic", "Forest"},
		{"Imotekh the Stormlord", "Ornithopter"},
		{"Insidious Roots", "Grizzly Bears"},
		{"On Wings of Gold", "Forest"},
		{"Owlin Historian", "Forest"},
		{"Quintorius, Field Historian", "Forest"},
		{"Rot Farm Mortipede", "Grizzly Bears"},
		{"Skeleton Crew", "Grizzly Bears"},
		{"Soul Enervation", "Grizzly Bears"},
		{"Spirit Mascot", "Forest"},
		{"Stonebound Mentor", "Forest"},
		{"Teval, the Balanced Scale", "Forest"},
		{"Thran Vigil", "Grizzly Bears"},
		{"Undead Hand Ninja", "Grizzly Bears"},
	}
	for _, tc := range cases {
		t.Run(tc.carrier, func(t *testing.T) {
			sc, id := newLeftGraveyardScen(t, []cardset.Entry{
				{Name: tc.carrier, Count: 1}, {Name: tc.card, Count: 1},
				{Name: "Grizzly Bears", Count: 1}, {Name: "Ornithopter", Count: 1},
				{Name: "Forest", Count: 2}, {Name: "Plains", Count: 8},
			})
			c := sc.perm(id[tc.carrier], 0)
			ab := trigLeftgraveyardGraveAb(t, sc, id[tc.carrier])
			sc.s.active = 0
			sc.s.npend = 0
			sc.s.detach(sc.grave(id[tc.card], 0, 1)[0])
			if n := batchPend(sc, c, ab); n != 1 {
				t.Fatalf("%s queued %d triggers, want 1", tc.carrier, n)
			}
		})
	}
}

// TestEnterFromGraveyardDoesNotSeeOwnLeave: a card returned from a graveyard
// does not see itself leave: its "cards leave your graveyard" ability triggers
// only from the battlefield and looks back to the board before the move, where
// it was a card in the graveyard (CR 603.10a), whichever effect moved it. A
// hidden search (Bind to Life's "a creature card milled this way") enters
// through plain enter.
func TestEnterFromGraveyardDoesNotSeeOwnLeave(t *testing.T) {
	sc, id := newLeftGraveyardScen(t, []cardset.Entry{
		{Name: "Garrison Excavator", Count: 1}, {Name: "Forest", Count: 2}, {Name: "Plains", Count: 8},
	})
	o := sc.grave(id["Garrison Excavator"], 0, 1)[0]
	sc.s.npend = 0
	from := sc.s.detach(o)
	sl := sc.s.enter(sc.k, o, 0, false, from)
	if sc.s.perms[sl].obj != o {
		t.Fatal("the card did not enter")
	}
	n := 0
	for i := uint8(0); i < sc.s.npend; i++ {
		if sc.s.pend[i].obj == o {
			n++
		}
	}
	if n != 0 {
		t.Fatalf("the returned card queued %d triggers, want 0", n)
	}
}
